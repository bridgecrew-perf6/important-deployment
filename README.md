# important-deployment operator

Watches deployments in `devops` namespace with label `importantDeployment: some-ci-system`. 

Every time an `importantDeployment` is deployed into this namespace, you have to ensure that an external services (could be https://httpbin.org running locally in a Docker) receives a notification about the name of the changed deployment-resource + the changes which were made.

There should be 3 types of notifications for deployments:  
a. when a deployment is freshly created  
b. when a deployment is ready (all replicas up and running)  
c. when a deployment is deleted

## Requirements:

* Go version go1.17.*

* kubectl

* Kubernetes cluster. For example, k3d or any remote cluster


## Deploy the important deployment operator

1. Deploy the CRDs, RBACs and the controller:  
```console
$ make deploy
```

2. Check if the controller is running successfully with:  
```console
$ kubectl get pods
```

## Architecture

![important-deployment](https://user-images.githubusercontent.com/13185122/163692491-9be9b5e0-6808-4d1a-b437-8f1443dc9fa6.jpg)

## Testing

Create a Kubernetes deployment resoruce in "devops" namespace with a label "importantDeployment: some-ci-system" or without it to see whether operator reacts on it. 
To see the how operator reacts, please have a look at the operator logs with
```console
$ kubectl logs -f -c manager {POD_NAME} -n {OPERATOR_NAMESPACE}
``` 

Moreover, you can also have look at the Notification CR which stores last sent notification to an external service.

### Notification types:
* CREATE: you see a log entry similar in the log where external httbin endpoint echoes the request:

```
{
  "args": {},
  "data": "{\"deploymentname\":\"devops/nginx-deployment\",\"message\":\"Created the deployment devops/nginx-deployment\"}",
  "files": {},
  "form": {},
  "headers": {
     ...
  },
  "json": {
    "deploymentname": "devops/nginx-deployment",
    "message": "Created the deployment devops/nginx-deployment"
  },
  "origin": "155.56.44.140",
  "url": "https://httpbin.org/post"
}
```

* UPDATE: you see the following where `replicas` of a deployment is changed from 2 to 10:

```

{
  "args": {},
  "data": "{\"deploymentname\":\"devops/nginx-deployment\",\"message\":\"Updated the deployment devops/nginx-deployment with: diff.Changelog{diff.Change{Type:\\\"update\\\", Path:[]string{\\\"Replicas\\\"}, From:2, To:10, ...",
  "files": {},
  "form": {},
  "headers": {
     ...
  },
  "json": {
    "deploymentname": "devops/nginx-deployment",
    "message": "Updated the deployment devops/nginx-deployment with: diff.Changelog{diff.Change{Type:\"update\", Path:[]string{\"Replicas\"}, From:2, To:10, ..."
  },
  "origin": "...",
  "url": "https://httpbin.org/post"
}

```


* READ: you see something similar as it follows:

```
{
  "args": {},
  "data": "{\"deploymentname\":\"devops/nginx-deployment\",\"message\":\"The deployment devops/nginx-deployment is ready.\"}",
  "files": {},
  "form": {},
  "headers": {
     ...
  },
  "json": {
    "deploymentname": "devops/nginx-deployment",
    "message": "The deployment devops/nginx-deployment is ready."
  },
  "origin": "...",
  "url": "https://httpbin.org/post"
}
```

* DELETE: you see something similar as it follows:

```
{
  "args": {},
  "data": "{\"deploymentname\":\"devops/nginx-deployment\",\"message\":\"The deployment devops/nginx-deployment is deleted.\"}",
  "files": {},
1.6501454516470113e+09  INFO    controller.deployment   The notification is sent successfully:  {"reconciler group": "apps", "reconciler kind": "Deployment", "name": "nginx-deployment", "namespace": "devops"}
  "form": {},
  "headers": {
    ...
  },
  "json": {
    "deploymentname": "devops/nginx-deployment",
    "message": "The deployment devops/nginx-deployment is deleted."
  },
  "origin": "155.56.44.140",
  "url": "https://httpbin.org/post"
}
```

