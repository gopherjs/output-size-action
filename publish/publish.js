// This script is meant to be executed in the actions/github-script context.
// 
// Using github-script allows us to use GitHub API without writing a proper JS
// package using GitHub's actions/toolkit library or having to worry about
// dependencies.
// 
// It would be an interesting exercise to rewrite this script in GopherJS, but
// at the moment interaction with other node_libraries in GopherJS is a bit
// awkward, so I didn't bother.
const fs = require('fs');

const commentMarker = '#outputSize';

function isSizeReport(comment) {
  return comment.body.includes(commentMarker) && comment.user.login == "github-actions[bot]"
}

function hideComment(github, node_id) {
  const query = `
    mutation($comment:ID!) {
      minimizeComment(input: {classifier: OUTDATED, subjectId: $comment}) {
        minimizedComment {
          isMinimized
        }
      }
    }
  `;
  return github.graphql(query, { 'comment': node_id });
}

module.exports = async (github, context, core, exec, artifactName) => {
  // Step 1: Verify that the action is used in the correct context.
  if (!context.payload.workflow_run) {
    core.error(`This action should be used in a workflow with "workflow_run" trigger, got: ${context.event}.`);
    return;
  }

  const workflow_run = context.payload.workflow_run;
  if (!workflow_run.pull_requests) {
    core.info(`The original workflow ${workflow_run.name} is not associated with pull requests, nowhere to post a comment to.`)
    return;
  }
  if (workflow_run.conclusion != "success") {
    core.info(`Report can only be published for a successful workflow, got: ${workflow_run.conclusion}.`);
    return;
  }

  // Step 2: Find the Markdown-formatted report artifact.
  const artifacts = await github.rest.actions.listWorkflowRunArtifacts({
    owner: context.repo.owner,
    repo: context.repo.repo,
    run_id: workflow_run.id,
  });
  const reports = artifacts.data.artifacts.filter((artifact) => artifact.name == artifactName);
  if (!reports) {
    core.info(`No report artifacts found, nothing to post.`);
    return;
  }

  // Step 3: Download and unpack report artifact.
  const download = await github.rest.actions.downloadArtifact({
    owner: context.repo.owner,
    repo: context.repo.repo,
    artifact_id: reports[0].id,
    archive_format: 'zip',
  });
  fs.writeFileSync('report.zip', Buffer.from(download.data));
  await exec.exec('unzip', 'report.zip');
  const report = fs.readFileSync('report.md', 'utf-8');

  // Step 4: Identify associated PRs.
  const prs = workflow_run.pull_requests.map((pr) => pr.number);
  core.info(`Associated pull requests: ${prs}`);

  for (const pr of prs) {
    // Step 5: Hide previous reports, which are now obsolete.
    const comments = await github.rest.issues.listComments({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: pr,
    });
    await Promise.all(comments.data.filter(isSizeReport).map((c) => hideComment(github, c.node_id)));

    // Step 6: Publish fresh report.
    const comment = await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: pr,
      body: `${report}\n\n${commentMarker}`,
    });
    core.info(`Commented at ${comment.data.html_url}`);
  }
};