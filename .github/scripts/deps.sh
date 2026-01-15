#!/usr/bin/env bash

function call_update_and_push_if_required() {
  local CURRENT=$1
  local LATEST=$2
  local BRANCH=$3
  local COMMIT_MSG=$4

  echo "check: ${CURRENT} == ${LATEST}, branch: ${BRANCH}"

  if [[ "${CURRENT}" != "${LATEST}" ]] && ! git branch --remotes | grep --perl-regexp "^\s+origin/${BRANCH}$"; then
    git checkout -b "${BRANCH}"

    update "${CURRENT}" "${LATEST}" | xargs --no-run-if-empty git add

    git commit -m "${COMMIT_MSG}"
    git push origin "${BRANCH}"
  fi
}