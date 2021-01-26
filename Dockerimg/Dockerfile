FROM busybox:stable
COPY podlogreader /
ENTRYPOINT [ "/podlogreader" ]

# Arguments:
#   Without any argument, will only create/update the *role* 
#   With `--create-sa-and-rolebinding` will create/update the *role*, but also create *serviceaccount* and *rolebinding* (if any of the 2 exist, its left unchanged)
#CMD ["--create-sa-and-rolebinding"]


