# podlogreader

A k8s-controller that makes possible for a serviceaccount to **read logs from specific deployment-pods** (and no other deployments or pods)

For a deployment to use it, it should add the label `podlogreader-affiliate: enable` in its pods-spec.

It will create in the namespace a serviceaccount, rolebinding and role with minimal permitions to read the logs of only those pods of that deployment. It will then keep always in-sync that role with any deployment-pods changes.

So the overall effect is to have a serviceaccount, for that deployment, that can read deployment-pods logs (and only those of the deployment, no other!), and which is resilient to deployment pod changes (replicas increased/decresed, pods deleted/created, etc...)  


In all honesty, this was a successfull intent to try and see if this whole idea was even possible to be accomplished :)

The code is very simple, and all runs in an independent process without interference with other internal kubernetes control-loops or controllers (ie, its uncomplicated, self-contained, and looks safe towards overall cluster operation :) ) 



## Install podlogreader-controller
```
git clone https://github.com/zipizap/podlogreader.git

helm upgrade --install --atomic podlogreader-controller Chart/

kubectl -n podlogreader-controller logs podlogreader-controller
```


## Use it with a deployment

In a deployment (ex: mydeployment) add the label `podlogreader-affiliate: enable` into its **.spec.template.metadata.labels**. This can be done in a new deployment or in an existing deployment in which case its pods will be restarted (because theirs podSpec changed which triggers recreatino of pods)

The controller will detect the label, and create/update (in same namespace) a role `podlogreader-mydeployment` containing minimum permitions to allow reading the logs of that deployment-pods. If the controller was deployed with argument "--create-sa-and-rolebinding", it will also create a serviceaccount and rolebinding with the role created. 


The deployment


## Uninstall podlogreader-controller

TODO




## It sounds like magic... How can this work?

In essence this is a "kubernetes custom-controller", that efficiently reacts on events of creation/update of pods-of-a-deployment, which contain the label "podlogreader-affiliate: enable", and creates a serviceaccount with minimal role permitions to only read *that* deployment-pods/log. It then keeps updating the role to allow reading access to the logs of the deployment-pods, as they are appended, deleted, changed... 

Was implemented from a good-looking controller-example, which uses the SharedInformer/Queue pattern (recommended as an efficient way to build custom-controllers for optimal caching of object changes)

Works by monitoring events of pods CREATE/UPDATE'ing in all namespaces, and checking if a pod contains the label "podlogreader-affiliate: enable", in which case:

  - Discovers the *deployment* owner of that pod (if exists) (ex: nginxdeploy)

  - Discovers the *name-of-all-pods-of-that-deployment* (ex: nginxdeploy-xxx-yy1, nginxdeploy-xxx-yy2, ..., nginxdeploy-xxx-yyn)

  - Creates-or-updates a *role* (ex: podlogreader-nginxdeploy) to read logs of only those deployment-pods (minimum permitions)

    It updates the role field `resourceNames:` with the *name-of-all-pods-of-that-deployment*, and keeps updating them on any pod-changes detected

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
  
    This creation of "serviceaccount" is optional and only happens when argument "--create-sa-and-rolebinding" is used

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

    This creation of "rolebinding" is optional and only happens when argument "--create-sa-and-rolebinding" is used

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

  
  

The controller runs in its separate process, (typically in a pod, but could also be an off-cluster process during development), that keeps a live-connection to the kubernetes api-server to be fed back events when a pod is create/updated/deleted (via SharedInformer for efficient caching). When an event-of-interest happens, the custom-controller reacts in handler-functions which can make additional calls to the api-server to create/update other intended cluster resources. 

Even though I did not detected any misbehaviour of the controller, it was made with care so that if someday an unexpected bug happens in the controller, it will just crash on its own process, and not affect any other cluster operations. Ie, it never delays or affects internal kubernetes-loops, or the processing of new resources, or any other existing controller's loops, as it works independently aislated from them all. This is an intended safeguard (ex: an admission mutating webhooks can introduce delay into internal-loops, but not a custom-controller like this one)   

The code is very simple - the essence is in the handler.go functions, the hard-part was understanding how to use the informer and queues and the client-go libraries... 

It helped a lot to follow some conference presentations, and the "programming kubernetes" ebook - see [Great talks and links] section further bellow. Also was very helpfull to build it from a good example that already included much of the complex lib function-wiring already in place (NOTE: the client-go version used is not the lattest one, but should work fine with all recent kubernetes-versions, as it only uses api-objects which are "v1" stable and does not use any "alpha/beta" objects)





## Great talks and links

This controller is derived from https://github.com/trstringer/k8s-controller-core-resource and its great blog-post explanations https://trstringer.com/extending-k8s-custom-controllers/ :)


It was much more lengthy to achieve than I expected... had to read and see lots of videos:
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




## Tinker with it yourself :)



### Edit source files, and run it locally (off-cluster, not in pod) for quick local testing
```
git clone https://github.com/zipizap/podlogreader.git
go get -u github.com/golang/dep/cmd/dep
go install github.com/golang/dep/cmd/dep
cd podlogreader
dep ensure
./go_run.sh
  # It will read existing ~/.kube/config file and connect to current context (cluster/user), which should have enough permitions ("cluster-admin" 
  # is excessive but will always work) to monitor pod-changes in all namespaces and create roles in any namespace
  # 
  # Stop it at anytime with CTRL-C
  #
```


### To compile and build a docker image, do:
```
# Edit compileBinary_buildAndPushDockerImage.sh 
#   - DOCKER_USERNAME and DOCKER_IMGwTAG around LOC45
#   - if desired, disable debug-messages by commenting out LOC15 `set -o xtrace`
vi ./compileBinary_buildAndPushDockerImage.sh

# Edit Dockerfile:
#  


# And then execute:
./compileBinary_buildAndPushDockerImage.sh

# In the end, the docker image is updated in the registry
# Now you could update the helm-chart to use this image and redeploy it :)

```


### Usefull development tips

- Show all REST-API calls made by each kubectl command
  ```
  # -v=9 shows all curl requests invoked against API-server
  kubectl get pods -A -v=9
  ```

- Make REST-API calls with kubectl, to let it take care of the authentication:
  ```
  kubectl --raw /apis/batch/v1
  ``` 

_____


## TODO
  - [x] make "creation of sa and rolebining" optional via argument: --create-sa-and-rolebinding
  - [x] create `compileBinary_buildAndPushDockerImage.sh` to show how to compile the golang static-binary, and then build its docker image
  - [ ] create **helm-chart** to:
      [x] create a _namespace_ 
      [x] create a _serviceaccount_ 
      [x] create a _rolebinding_ of the serviceaccount with role cluster-admin
      [x] create a _deployment_, with only 1 replica, associated with the serviceaccount
  - [ ] Change helm-chart, to use for the controller, a more restricted role instead of the cluster-admin role
  - [ ] document how to deploy the helm-chart, with a **demo**   

