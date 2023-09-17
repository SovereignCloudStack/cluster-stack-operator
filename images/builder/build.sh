#!/bin/sh

# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu

SRC_PATH=/src/cluster-stack-operator

uid=$(stat --format="%u" "${SRC_PATH}")
gid=$(stat --format="%g" "${SRC_PATH}")
echo "cso:x:${uid}:${gid}::${SRC_PATH}:/bin/bash" >>/etc/passwd
echo "cso:*:::::::" >>/etc/shadow
echo "cso	ALL=(ALL)	NOPASSWD: ALL" >>/etc/sudoers

su cso -c "PATH=${PATH} make -C ${SRC_PATH} BUILD_IN_CONTAINER=false $*"
