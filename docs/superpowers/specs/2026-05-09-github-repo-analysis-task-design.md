# GitHub Repo Analysis Task Design

## Goal

Create a dedicated GitHub repository analysis task flow. When a user starts a task with a GitHub repository URL, the app creates a task-bound conversation, jumps to that conversation, automatically sends a generated prompt, analyzes the repository, and saves the final Code Wiki as a Markdown artifact.

## Interaction

1. The user enters a GitHub repository URL and a request such as "generate a structured Code Wiki".
2. The app calls `POST /v1/runs`.
3. Go detects the GitHub repository URL, creates a `github_repo_analysis` Run, creates a task chat session, and returns `taskSessionId` plus `taskPrompt`.
4. The app navigates to the chat screen, selects the task session, and automatically sends `taskPrompt`.
5. The task session is hidden from normal recent chat history. It is reachable from Run history and Run detail.
6. When the first task message is processed, Go fetches and scans the repository, asks the LLM to produce the Code Wiki when a model is configured, stores the Markdown as a `code_wiki` Run artifact, and streams the result back to the task conversation.

## Responsibility Split

Go owns product workflow and deterministic repository handling:

- GitHub URL detection and normalization.
- Run and task session creation.
- Session history classification and filtering.
- Public repository clone/download into a temporary workspace.
- File filtering, file tree generation, language/framework hints, manifest detection, key file selection, and safe cleanup.
- Run step status, artifact persistence, and SSE/chat streaming integration.
- Fallback Code Wiki generation when no LLM backend is configured.

LLM owns synthesis:

- Transform repository scan summaries and selected file excerpts into a coherent Code Wiki.
- Explain architecture, modules, classes/functions, dependencies, run commands, and extension points.
- Surface uncertainty when files were skipped or repository limits were reached.

## Data Model

- `runs.kind`: `research` or `github_repo_analysis`.
- `runs.source_url`: normalized GitHub repository URL.
- `runs.task_session_id`: chat session used for the task conversation.
- `runs.task_prompt`: generated first message sent by the app.
- `chat_sessions.kind`: `chat` or `task`.
- `chat_sessions.run_id`: owning Run for task sessions.
- `run_artifacts.kind`: add `code_wiki`.

## Backend Flow

`CreateRun` detects GitHub URLs in `goal`. For GitHub tasks it creates the Run, creates a task session, stores the generated prompt, and skips queue dispatch. The task execution is driven by the automatic chat message in the task session.

`chat.StreamConversation` checks whether the selected session is a task session. If it belongs to a GitHub analysis Run and the Run is not completed, it delegates to the Run task conversation executor instead of the normal generic chat path.

The GitHub executor creates deterministic Run steps:

- `github_repo_fetch`
- `github_repo_scan`
- `code_structure_analyze`
- `code_wiki_writer`
- `finalize`

## Frontend Flow

`ComposeScreen` keeps using `api.createRun`. If the returned Run has `taskSessionId` and `taskPrompt`, it routes to the chat tab with these route params. `RunsScreen` consumes those params once, activates the task session, and sends the prompt automatically through the existing chat streaming UI.

The regular session drawer receives only normal chat sessions from the API because the backend filters task sessions from `ListChatSessions`.

## Documentation

Update the technical design docs to describe the new GitHub Repo Analysis flow, Go/LLM responsibility split, data fields, and user interaction.
