# az-whose

A tool to find out the owner of Azure resources by inspecting the activity logs.

## Why

Azure users have no easy way to identify the *owner* of a certain resource/resource group, since Azure doesn't record it in the resource's metadata. Azure does have the "owner" role though, which is the Role-Based Access Control (RBAC) to manage permissions for the resources, that is not necessarily the real *owner* of the resource.

> NOTE: The term "owner" used here and below means the user/application that created the resource, or frequently send API to operate this resource.

A tool to answer the basic question like "who is the owner of a resource group" should have been there long ago.

## How

By searching the answer to this basic question, the search engine brings me to [this](https://learn.microsoft.com/en-us/answers/questions/971455/how-can-i-find-out-who-created-a-particular-resour), which inspects the [activity logs](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-insights) (retained for 90 days) and find the one that most likely is the *owner*.

This tool follows the above practice, and provide an easy way to do the inspection for you. The stragety used here is as simple as below:

- Users specify either a resource id, or a filter with resource group name or/and resource types, to search for the activity logs (within 90 days)
- Aggregate the logs by resource id, operator/caller, operation type (i.e. write, action, delete). Note that if there is a "delete" and it is not the last operation, it will reset all the operation record detected so far. The sanity here is to only record the latest lifecycle of this resource. If you'd like to keep the operations including the delete, use option `--keep-delete`. 
- Each operation type has a weight, which will be used to factor with the count of the operations, to get a confidence score
- For each resource, the operators are sorted with the confidence score
- Print the result 

## Example

```shell
❯ az-whose -f -s <sub-id> --resource-group-name <rg-name>
{
  "/SUBSCRIPTIONS/<sub-id>/RESOURCEGROUPS/<rg-name>": {
    "id": "/SUBSCRIPTIONS/<sub-id>/RESOURCEGROUPS/<rg-name>",
    "stats": [
      {
        "caller": {
          "type": "user",
          "name": "magodo@foo.com"
        },
        "score": 10,
        "total": 20,
        "details": {
          "write": 1,
          "action": 0,
          "delete": 0
        }
      },
      {
        "caller": {
          "type": "app",
          "name": "magodo-app",
          "object_id": "xxxx",
          "owners": ["magodo@foo.com"]
        },
        "score": 10,
        "total": 20,
        "details": {
          "write": 1,
          "action": 0,
          "delete": 0
        }
      }
    ]
  },
  "/SUBSCRIPTIONS/<sub-id>/RESOURCEGROUPS/<rg-name>/PROVIDERS/MICROSOFT.CONTAINERREGISTRY/REGISTRIES/<acr-name>": {
    "id": "/SUBSCRIPTIONS/<sub-id>/RESOURCEGROUPS/<rg-name>/PROVIDERS/MICROSOFT.CONTAINERREGISTRY/REGISTRIES/<acr-name>",
    "stats": [
      {
        "caller": {
          "type": "user",
          "name": "magodo@foo.com"
        },
        "score": 32,
        "total": 43,
        "details": {
          "write": 3,
          "action": 2,
          "delete": 0
        }
      },
      {
        "caller": {
          "type": "app",
          "name": "magodo-app",
          "object_id": "xxxx",
          "owners": ["magodo@foo.com"]
        },
        "score": 11,
        "total": 43,
        "details": {
          "write": 1,
          "action": 1,
          "delete": 0
        }
      }
    ]
  }
}
```
