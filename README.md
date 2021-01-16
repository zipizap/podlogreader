# podlogreader

A k8s-controller that makes possible for a serviceaccount to **read logs of a specific deployment-pods** (and no other deployments or pods)
For a deployment to use it, it should add the label `podlogreader-affiliate: enable` in its pods-spec.
It just keeps updating a role, with "resoureNames:" containing that specific deployment-pods names


This was a successfull intent to try and see if this idea was even possible to do :)
The code is very simple, and all runs in an independent process without interference with any internal kubernetes control-loops or other controllers (ie, its uncomplicate and looks safe :) ) 


## It sounds magic... How can this work?

In essence this is a custom-controller, that efficiently reacts on the creation/update of pods-of-a-deployment with label "podlogreader-affiliate: enable", and creates a serviceaccount with minimal role permitions to only read *that* deployment-pods/log. It then keeps updating the role to allow reading access of the logs of the deployment-pods, as they are appended, deleted, changed... 

Was implemented from a good-looking controller-example, which uses the SharedInformer/Queue pattern (recommended as the right way to do custom-controllers for optimal caching of events)

Works by monitoring events of pods CREATE/UPDATE'ing in all namespaces, and checking if a pod contains the label "podlogreader-affiliate: enable", in which case:
  - Discovers the *deployment* owner of that pod (if exists) (ex: nginxdeploy)

  - Discovers the *name-of-all-pods-of-that-deployment* (ex: nginxdeploy-xxx-yy1, nginxdeploy-xxx-yy2, ..., nginxdeploy-xxx-yyn)

  - Creates-or-updates a *role* (ex: podlogreader-nginxdeploy) to read logs of only those deployment-pods (minimum permitions)
    It updates the role field `resouceNames:` with the *name-of-all-pods-of-that-deployment*, andd always keepis updating it on any pod-changes
    Ex:
    ```
    apiVersion: rbac.authorization.k8s.io/v1
    kind: Role
    metadata:
      namespace: thenamespaceofthedeployment
      name: podlogreader-nginxdeploy
    rules:
    # minimal pod permitions necesary, to enable pods/log by label
    - apiGroups: [""]
      resources: ["pods"]
      verbs: ["list"]
    - apiGroups: [""]
      resources: ["pods/log"]
      resourceNames: ["nginxdeploy-69db9bf477-2rt7x", "nginxdeploy-69db9bf477-4fg6h"]
      verbs: ["get", "list", "watch"]
    ```

  - Creates a *serviceaccount* (ex: podlogreader-nginxdeploy). 
    If serviceaccount already exists, its left unchanged    
    Ex:
    ```
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      namespace: thenamespaceofthedeployment
      name: podlogreader-nginxdeploy
    ```    

  - Creates a *rolebinding* (ex: podlogreader-nginxdeploy) to bind the role with the serviceaccount. 
    If rolebinding already exists, its left unchanged    
    Ex:
    ```
    apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    metadata:
      name: podlogreader-nginxdeploy
      namespace: thenamespaceofthedeployment
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: Role
      name: podlogreader-nginxdeploy
    subjects:
    - kind: ServiceAccount
      name: podlogreader-nginxdeploy
      namespace: thenamespaceofthedeployment
    ```


The controller runs in its process, typically in a pod, and keeps a conection to the api-server watching for events, and when an event-of-interest happens it reacts by making additional calls to the api to create/update necesary cluster resources. So if anything unexpected happens in the controller - like a bug - the controller process will just crash and never delay of affect the internal kubernetes-loop or other controller's loops.

The code is very simple - the essence is in the handler.go functions




## Install
```
go get -u github.com/golang/dep/cmd/dep
go install github.com/golang/dep/cmd/dep
dep ensure
```



## Great talks and links

This is all forked from https://github.com/trstringer/k8s-controller-core-resource and its great blog-post explanations https://trstringer.com/extending-k8s-custom-controllers/ :)


It was much more lengthy to achieve than I expected... had to read and see lots of videos, around:
 - Admission controller, mutating webhooks: these did not seem a good fit for this, and they can interfere with internal control-loop of kubernetes - yaikes! 
 - custom controller: what this is in fact
 - client-go (official kubernetes client library in go)
 - Kubernetes API-Server, REST-API 
 - golang: needed to brush up the rusty-notes :)


Also, another big thanks to the youtube conference videos (search "kubernetes custom controller"), specially from Alena Prokharchyk, Lili Cosic and Aaron Levy:
  - https://www.youtube.com/watch?v=QIMz4V9WxVc
  - https://www.youtube.com/watch?v=Q88kI8X5R48
  - https://www.youtube.com/watch?v=_BuqPMlXfpE


The book "Programming Kubernetes" was also very helpfull with its clear explanations of the so-many pieces that are juggled around to make a "custom controller" 

This relevant discussion about how a role can manage permitions of pods/log, per namespace or per pod-name, but not per deployment-name https://github.com/kubernetes/kubernetes/issues/56582




## Usefull development tips

Show all REST-API calls made by each kubectl command
```
# -v=9 shows all curl requests invoked against API-server
kubectl get pods -A -v=9
```

Make REST-API calls with kubectl, to let it take care of the authentication:
```
kubectl --raw /apis/batch/v1
``` 

