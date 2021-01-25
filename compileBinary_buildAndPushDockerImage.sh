#!/usr/bin/env bash
# Paulo Aleixo Campos
__dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
function shw_info { echo -e '\033[1;34m'"$1"'\033[0m'; }
function error { echo "ERROR in ${1}"; exit 99; }
trap 'error $LINENO' ERR
#exec > >(tee -i /tmp/$(date +%Y%m%d%H%M%S.%N)__$(basename $0).log ) 2>&1
set -o errexit
  # NOTE: the "trap ... ERR" alreay stops execution at any error, even when above line is commente-out
set -o pipefail
set -o nounset
export PS4='\[\e[44m\]\[\e[1;30m\](${BASH_SOURCE}:${LINENO}):${FUNCNAME[0]:+ ${FUNCNAME[0]}():}\[\e[m\]      '

# comment next line to disable bash debug messages of each function/statement executed 
set -o xtrace


compileBinary() {
  cd podlogreader
  go get -u github.com/golang/dep/cmd/dep
  go install github.com/golang/dep/cmd/dep
  dep ensure
  CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o ../bin/podlogreader .
  cd ..
}

buildAndPushDockerImage() {
  cd Dockerimg
  cp -vf ../bin/podlogreader .
  docker build . --tag $DOCKER_IMGwTAG
  
  # :) this was usefull for testing
  ## docker run -it  -v ~/mod_kube:/root/.kube zipizap/podlogreader-controller:0.0.1

  # docker login 
  docker push $DOCKER_IMGwTAG
  cd ..
}


main() {
  cd "${__dir}"
   
  # Put here your username and intented image:tag
  DOCKER_USERNAME="zipizap"
  DOCKER_IMGwTAG="$DOCKER_USERNAME/podlogreader-controller:0.0.1"


  shw_info ">> Compiling go binary: bin/podlogreader"
  compileBinary

  shw_info ">> Building and pushing docker image: $DOCKER_IMGwTAG"
  buildAndPushDockerImage

  shw_info ">> Execution completed successfully"

}
main "$@"