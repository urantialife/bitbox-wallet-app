#!/bin/bash -e
# Copyright 2018 Shift Devices AG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script has to be called from the project root directory.
go build ./...
go test ./...

golangci-lint run

yarn --cwd=frontends/web install # needed to install the eslint dev dep.
make weblint
yarn --cwd=frontends/web test --ci --no-color --coverage
# check that the i18n files are formatted correctly (avoids noisy diff when
# pulling from locize)
if ! locize format frontends/web/src/locales --format json --dry true ; then
    echo "i18n files malformatted. Fix with: make locize-fix"
    exit 1
fi
