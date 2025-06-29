# **SCIM Integration Mediator \- User Guide**

## **1\. Overview**

The SCIM Integration Mediator is a command-line application designed to provide a reliable and auditable way to manage the identity lifecycle (users and groups) for a SmartSuite tenant. It acts as a "Trusted Mediator" that sits between your identity data sources and the SmartSuite SCIM API.

Its key features include:

* A local **System of Record** that mirrors the state of users and groups in SmartSuite, providing a "Rosetta Stone" between your internal user IDs and SmartSuite's internal IDs.  
* A detailed **Audit Log** that records every action taken by the application.  
* Resumable **batch processing** for handling large-scale updates.  
* A **graceful shutdown** mechanism to prevent data loss if the application is interrupted.

## **2\. Configuration**

The application is configured entirely through environment variables.

| Variable | Description | Example |
| :---- | :---- | :---- |
| SMARTSUITE\_API\_URL | **Required.** The base URL for the SmartSuite SCIM API. | https://app.smartsuite.com/authentication/scim |
| SMARTSUITE\_API\_KEY | **Required.** The bearer token for authentication. | your\_secret\_api\_key |
| DATA\_DIR | *Optional.* The directory to store state files (users.json, groups.json, audit.log). | Defaults to ./data |

## **3\. Installation**

The application is a single binary built from the Go source code.

\# Navigate to the project's root directory  
go build \-o scim-mediator .

This will create a scim-mediator executable in the current directory.

## **4\. Command Reference**

All operations are performed using sub-commands.

### **populate**

**Purpose:** Performs the initial "Discovery & Adoption" to build the local System of Record. This command should be run **once** during the initial setup. It will overwrite any existing local data.

**Usage:**

./scim-mediator populate

### **refresh**

**Purpose:** Reconciles the local System of Record with the live state in SmartSuite. It checks for any users or groups that were created, updated, or deleted directly in SmartSuite (outside of the mediator) and logs these discrepancies.

**Usage:**

./scim-mediator refresh

This command is safe to run multiple times and is recommended for periodic reconciliation.

### **create-user**

**Purpose:** Provisions a single new user in SmartSuite from a JSON file.

**Process:** This command first performs a "search-before-insert" validation. It queries the live SmartSuite API using a filter to ensure no user with the given userName already exists. Only after confirming the user is unique does it proceed with the creation.

**Usage:**

./scim-mediator create-user \--from-file ./path/to/new\_user.json

**Flag:**

* \--from-file \<path\>: **Required.** Path to the JSON file containing the new user's attributes.

### **create-group**

**Purpose:** Provisions a single new group (team) in SmartSuite from a JSON file.

**Usage:**

./scim-mediator create-group \--from-file ./path/to/new\_group.json

**Flag:**

* \--from-file \<path\>: **Required.** Path to the JSON file containing the new group's name.

### **manage-group-members**

**Purpose:** Adds or removes members from an existing group.

**Usage:**

./scim-mediator manage-group-members \--group "Engineers" \--add "user1@example.com" \--remove "user2@example.com"

**Flags:**

* \--group \<name\>: **Required.** The name of the group to manage.  
* \--add \<eppn\>: A user's ePPN to add. Can be specified multiple times.  
* \--remove \<eppn\>: A user's ePPN to remove. Can be specified multiple times.

### **process-batch**

**Purpose:** Executes a series of tasks (updates, deactivations, group changes) from a single source file. This command is resumable; if it is interrupted, it can be re-run to complete the remaining tasks.

**Usage:**

./scim-mediator process-batch \--from-file ./path/to/batch\_tasks.json

**Flag:**

* \--from-file \<path\>: **Required.** Path to the JSON file containing the list of tasks.

### **cleanup-users**

**Purpose:** Implements the "Two-Stage Farewell" for off-boarding. It scans for any users who were deactivated more than 7 days ago and permanently deletes them from SmartSuite to free up licenses.

**Usage:**

./scim-mediator cleanup-users

## **5\. Scheduling Recurring Tasks**

To keep the system synchronized and clean, two commands should be run on a schedule using a tool like cron.

* **refresh**: Recommended to run once a day to detect any manual changes.  
* **cleanup-users**: Recommended to run once a day (e.g., nightly) to enforce the 7-day grace period for deactivated users.

**Example Crontab Entries:**

\# Run the refresh command every day at 1 AM  
0 1 \* \* \* /path/to/scim-mediator refresh \>\> /var/log/scim-mediator.log 2\>&1

\# Run the user cleanup command every day at 2 AM  
0 2 \* \* \* /path/to/scim-mediator cleanup-users \>\> /var/log/scim-mediator.log 2\>&1

## **6\. Example Input Files**

This project includes a directory named /example\_files containing sample JSON files that demonstrate the correct format for the create-user, create-group, and process-batch commands.

These files can be used directly for testing or as templates for creating your own automation scripts.

### **Important Considerations When Using the Examples**

Before using the example files, you **must** modify them to reflect the users and groups in your own environment. Please consider the following:

* **Usernames and Emails:** All userName and email fields (e.g., j.doe@example.com) are placeholders. They must be replaced with valid usernames that correspond to your identity source.  
* **Group Names:** In files like batch\_tasks.json, any group name provided in the data field (e.g., "Engineers") must exactly match the displayName of a group that already exists in SmartSuite.  
* **Batch Task Targets:** The target field in batch\_tasks.json must reference a userName that exists in SmartSuite and is known to the Mediator's local store. Running the refresh command before a batch process is a good practice to ensure the local store is up-to-date.  
* **Data Consistency:** Ensure that all data within the files is accurate. For example, when updating a user, the target should be their *current* userName. If a previous batch job changed that username, the next batch file must use the new one.

## **Appendix: Example API curl Commands**

This section provides raw curl examples for interacting directly with the SmartSuite SCIM API. These are useful for testing and debugging. Remember to replace placeholders like BASE\_URL\_URL, API\_KEY, and example IDs/names with your actual values.

### **Search for a User by userName**

Used by the create-user command for validation.

curl \-X GET \\  
  'BASE\_URL\_URL/Users?filter=userName eq "j.doe@example.com"' \\  
  \--header 'Authorization: Bearer API\_KEY' \\  
  \--header 'Accept: application/scim+json'

### **Create a New User**

The underlying API call for the create-user command.

curl \-X POST \\  
  'BASE\_URL\_URL/Users' \\  
  \--header 'Authorization: Bearer API\_KEY' \\  
  \--header 'Content-Type: application/scim+json' \\  
  \--data-raw '{  
    "schemas": \["urn:ietf:params:scim:schemas:core:2.0:User"\],  
    "userName": "new.user@example.com",  
    "name": {  
      "formatted": "New User",  
      "familyName": "User",  
      "givenName": "New"  
    },  
    "emails": \[{  
      "primary": true,  
      "value": "new.user@example.com",  
      "type": "work"  
    }\],  
    "active": true  
  }'

### **Update a User's Attribute (e.g., Deactivate)**

Used by process-batch with a deactivate task.

curl \-X PATCH \\  
  'BASE\_URL\_URL/Users/USER\_SCIM\_ID\_HERE' \\  
  \--header 'Authorization: Bearer API\_KEY' \\  
  \--header 'Content-Type: application/scim+json' \\  
  \--data-raw '{  
    "schemas": \["urn:ietf:params:scim:api:messages:2.0:PatchOp"\],  
    "Operations": \[{  
      "op": "replace",  
      "path": "active",  
      "value": false  
    }\]  
  }'

### **Add a Member to a Group**

Used by manage-group-members and process-batch with an add-to-group task.

curl \-X PATCH \\  
  'BASE\_URL\_URL/Groups/GROUP\_SCIM\_ID\_HERE' \\  
  \--header 'Authorization: Bearer API\_KEY' \\  
  \--header 'Content-Type: application/scim+json' \\  
  \--data-raw '{  
    "schemas": \["urn:ietf:params:scim:api:messages:2.0:PatchOp"\],  
    "Operations": \[{  
      "op": "add",  
      "path": "members",  
      "value": \[{  
        "value": "USER\_SCIM\_ID\_TO\_ADD"  
      }\]  
    }\]  
  }'  
