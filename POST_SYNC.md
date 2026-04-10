Adds dev session crash recovery and extracts AI tool installation into a cached composite action.

## Features
- Adds recovery.md checkpoint file to dev sessions, enabling automatic resume after crashes or context loss with branch mismatch detection (#197)
- Updates session-init to detect and surface recovery.md on session start (#197)
- Extracts inline AI tool installation blocks in agentic-pipeline.yml into a reusable composite action with dependency caching (#204)
