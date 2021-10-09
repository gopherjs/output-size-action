#!/usr/bin/env bash

set -e;

command="$(basename "$0")";

if [[ "${command}" == "pre-commit" ]]; then
  echo "Pre-commit: queueing compiled code update.";
  touch ".post-commit-update";
elif [[ "${command}" == "post-commit" ]]; then
  if [ ! -e ".post-commit-update" ]; then
    exit 0;
  fi

  echo "Post-commit: checking if compiled code changed."
  rm .post-commit-update;
  gopherjs build -o _dist/measure.js ./measure;

  if git diff --quiet HEAD ./_dist; then
    echo "Post-commit: no changes."
    exit 0;
  fi

  echo "Post-commit: amending the commit with code changes."
  ls -lh _dist/*.js;
  git add _dist/*.js _dist/*.js.map;
  git commit --amend -C HEAD --no-verify;
else
  echo "Setting up git hooks.";
  ln -s -r -f "$0" .git/hooks/pre-commit;
  ln -s -r -f "$0" .git/hooks/post-commit;
fi