package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/smartsuite"
	"github.com/SmartSuiteFoundry/scim-mediator/pkg/store"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var processBatchCmd = &cobra.Command{
	Use:   "process-batch",
	Short: "Executes a bulk update from a source file.",
	Long: `Reads a source file containing a list of tasks (e.g., update, deactivate, add-to-group),
and processes them sequentially. This command is designed to be resumable; if it's
interrupted, it can be re-run to process the remaining pending tasks.`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- Get context for graceful shutdown ---
		ctx := cmd.Context()

		// --- Initialization ---
		fromFile, _ := cmd.Flags().GetString("from-file")
		slog.Info("Starting batch process", "from_file", fromFile)

		dataDir := viper.GetString("data_dir")
		if dataDir == "" {
			dataDir = "./data"
		}
		jobQueueFile := filepath.Join(dataDir, "job_queue.json")
		var jobQueue []models.JobTask

		// --- Prepare Job Queue ---
		if _, err := os.Stat(jobQueueFile); os.IsNotExist(err) {
			slog.Info("No existing job queue found. Creating one from source file.")
			sourceData, err := os.ReadFile(fromFile)
			if err != nil {
				slog.Error("Failed to read source file", "file", fromFile, "error", err)
				os.Exit(1)
			}
			if err := json.Unmarshal(sourceData, &jobQueue); err != nil {
				slog.Error("Failed to unmarshal batch tasks from source file", "error", err)
				os.Exit(1)
			}
			for i := range jobQueue {
				jobQueue[i].Status = "pending"
			}
		} else {
			slog.Info("Existing job queue found. Resuming process.")
			queueData, err := os.ReadFile(jobQueueFile)
			if err != nil {
				slog.Error("Failed to read existing job queue file", "error", err)
				os.Exit(1)
			}
			if err := json.Unmarshal(queueData, &jobQueue); err != nil {
				slog.Error("Failed to unmarshal job queue data", "error", err)
				os.Exit(1)
			}
		}

		// --- Process Job Queue ---
		client, err := smartsuite.NewClient(viper.GetString("api_url"), viper.GetString("api_key"))
		if err != nil {
			slog.Error("Failed to create API client", "error", err)
			os.Exit(1)
		}
		s, err := store.NewStore(dataDir)
		if err != nil {
			slog.Error("Failed to create store", "error", err)
			os.Exit(1)
		}
		userStore, err := s.LoadUsers()
		if err != nil {
			slog.Error("Failed to load user store", "error", err)
			os.Exit(1)
		}
		groupStore, err := s.LoadGroups()
		if err != nil {
			slog.Error("Failed to load group store", "error", err)
			os.Exit(1)
		}

		slog.Debug("Starting Queue.", "size", len(jobQueue))

		var tasksProcessed int
		hasChanges := false
		for i := range jobQueue {
			// --- Check for graceful shutdown signal ---
			if ctx.Err() != nil {
				slog.Warn("Shutdown signal received. Saving progress and exiting.", "reason", ctx.Err())
				saveQueue(jobQueueFile, jobQueue)
				return // Exit gracefully
			}

			task := &jobQueue[i]
			if task.Status != "pending" {
				slog.Debug("Not Pending.", "status", task.Status)
				continue
			}

			hasChanges = true
			slog.Debug("Processing task", "type", task.Type, "target", task.Target)

			var taskErr error
			switch task.Type {
			case "update":
				taskErr = handleUpdateTask(ctx, client, s, userStore, task)
			case "deactivate":
				taskErr = handleDeactivateTask(ctx, client, s, userStore, task)
			case "add-to-group":
				taskErr = handleGroupMembershipTask(ctx, client, userStore, groupStore, task, "add")
			case "remove-from-group":
				taskErr = handleGroupMembershipTask(ctx, client, userStore, groupStore, task, "remove")
			default:
				taskErr = fmt.Errorf("unknown task type: '%s'", task.Type)
			}

			if taskErr != nil {
				task.Status = "failed"
				logAndAudit(s, "ProcessBatch", task.Target, "error", "Task failed", "error", taskErr)
			} else {
				task.Status = "completed"
				logAndAudit(s, "ProcessBatch", task.Target, "info", fmt.Sprintf("Task '%s' completed successfully.", task.Type))
			}

			tasksProcessed++
			if tasksProcessed%5 == 0 {
				slog.Info("...Saving progress...", "progress", tasksProcessed)
				saveQueue(jobQueueFile, jobQueue)
			}
		}

		if hasChanges {
			saveQueue(jobQueueFile, jobQueue)
			slog.Info("Batch process finished.")
		} else {
			slog.Info("No pending tasks to process. Batch process complete.")
		}

		// --- Archive Job Queue on Success ---
		allCompleted := true
		for _, task := range jobQueue {
			if task.Status != "completed" {
				allCompleted = false
				slog.Warn("Not all tasks were completed successfully. Job queue will not be archived.", "task_target", task.Target, "task_status", task.Status)
				break
			}
		}

		if allCompleted && len(jobQueue) > 0 {
			timestamp := time.Now().Format("20060102-150405")
			completedFileName := fmt.Sprintf("%s.completed_%s", jobQueueFile, timestamp)
			slog.Info("All tasks completed successfully. Archiving job queue.", "new_name", completedFileName)
			if err := os.Rename(jobQueueFile, completedFileName); err != nil {
				slog.Error("Failed to archive completed job queue.", "error", err)
			}
		}
	},
}

// handleUpdateTask processes a single user attribute update task.
func handleUpdateTask(ctx context.Context, client *smartsuite.Client, s *store.Store, userStore map[string]models.UserRecord, task *models.JobTask) error {
	record, ok := userStore[task.Target]
	if !ok {
		return fmt.Errorf("user '%s' not found in local store", task.Target)
	}

	dataMap, ok := task.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("task data for update must be a map of attributes")
	}

	var operations []models.SCIMPatchOp
	for key, value := range dataMap {
		operations = append(operations, models.SCIMPatchOp{
			Op:    "replace",
			Path:  key,
			Value: value,
		})
	}

	if len(operations) == 0 {
		return fmt.Errorf("no update operations provided for user '%s'", task.Target)
	}

	// Perform the API call first.
	err := client.PatchUser(ctx, record.SCIMID, operations)
	if err != nil {
		return err
	}

	newUserName := ""
	for key, value := range dataMap {
		switch key {
		case "title":
			if title, ok := value.(string); ok {
				record.Title = title
			}
		case "userName":
			if un, ok := value.(string); ok {
				newUserName = un
			}
			// Add other attribute cases here as needed
		}
	}

	// If the userName (the key of our map) has changed, we must update the map.
	if newUserName != "" && newUserName != task.Target {
		// Delete the old record
		delete(userStore, task.Target)
		// Add the new record
		userStore[newUserName] = record
	} else {
		// Otherwise, just update the existing record
		userStore[task.Target] = record
	}

	return s.SaveUsers(userStore)
}

// handleDeactivateTask processes a single user deactivation task.
func handleDeactivateTask(ctx context.Context, client *smartsuite.Client, s *store.Store, userStore map[string]models.UserRecord, task *models.JobTask) error {
	record, ok := userStore[task.Target]
	if !ok {
		return fmt.Errorf("user '%s' not found in local store", task.Target)
	}
	operations := []models.SCIMPatchOp{{Op: "replace", Path: "active", Value: false}}
	err := client.PatchUser(ctx, record.SCIMID, operations)
	if err != nil {
		return err
	}
	now := time.Now()
	record.DeactivationTimestamp = &now
	record.Status = "inactive"
	userStore[task.Target] = record
	return s.SaveUsers(userStore)
}

// handleGroupMembershipTask processes adding or removing a user from a group.
func handleGroupMembershipTask(ctx context.Context, client *smartsuite.Client, userStore map[string]models.UserRecord, groupStore map[string]models.GroupRecord, task *models.JobTask, opType string) error {
	user, ok := userStore[task.Target]
	if !ok {
		return fmt.Errorf("user '%s' not found in local store", task.Target)
	}
	groupName, ok := task.Data.(string)
	if !ok {
		return fmt.Errorf("task data for group membership must be the group name (string)")
	}
	group, ok := groupStore[groupName]
	if !ok {
		return fmt.Errorf("group '%s' not found in local store", groupName)
	}
	var op models.SCIMPatchOp
	if opType == "add" {
		op = models.SCIMPatchOp{Op: "add", Path: "members", Value: []map[string]string{{"value": user.SCIMID}}}
	} else if opType == "remove" {
		op = models.SCIMPatchOp{Op: "remove", Path: fmt.Sprintf(`members[value eq "%s"]`, user.SCIMID)}
	} else {
		return fmt.Errorf("internal error: invalid opType '%s'", opType)
	}
	return client.PatchGroup(ctx, group.SCIMID, []models.SCIMPatchOp{op})
}

// saveQueue marshals and writes the job queue to a file to save progress.
func saveQueue(path string, queue []models.JobTask) {
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		slog.Warn("Could not marshal job queue to save progress", "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		slog.Warn("Could not write job queue file to save progress", "error", err)
	}
}

func init() {
	var fromFile string
	processBatchCmd.Flags().StringVar(&fromFile, "from-file", "", "Path to the JSON file containing batch tasks.")
	processBatchCmd.MarkFlagRequired("from-file")
}
