#! /usr/bin/env bash
set -e

if [ $# -ne 2 ]; then
  echo "need the version number and release comment as argument"
  echo "e.g. ${0} 0.4.5 'fix local modules and modules with install_path purging bug #80 #82'"
  echo "Aborting..."
  exit 1
fi
#
time go test -v
#
# Remove leading 'v' from the version number if present
version=${1#v}
#
if [ $? -ne 0 ]; then
  echo "Tests unsuccessful"
  echo "Aborting..."
  exit 1
fi
#
#
echo "creating git tag v${1}"
git tag v${1}
echo "pushing git tag v${1}"
git push -f --tags
git push

# try to get the project name from the current working directory
projectname=${PWD##*/}
upx=$(which upx)
export CGO_ENABLED=0
export BUILDTIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
export BUILDVERSION=$(git describe --tags)

build() {
  echo "building ${projectname}-$1-$2 with version ${version}"
  env GOOS=$1 GOARCH=$2 go build -ldflags "-X main.buildtime=${BUILDTIME} -X main.buildversion=${BUILDVERSION}"
  if [ ${#upx} -gt 0 ]; then
    if [ $1 == "linux" ]; then
      $upx ${projectname}
    fi
  fi
  zip ${projectname}-v${version}-$1-$2.zip ${projectname}
}

for os in darwin linux; do
  for arch in arm64 amd64; do
    build $os $arch
  done
done

test -z ${GITHUB_TOKEN} || echo "creating github release v${1}"
test -z ${GITHUB_TOKEN} && echo "skipping github-release as GITHUB_TOKEN is not set" || gh release create --fail-on-no-commits --verify-tag --repo ${projectname} --title "v${1}" --notes "${2}" v${1} ./${projectname}-v${1}*.zip
