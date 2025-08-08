## How to use this prompt template:
🎯 OPTIMAL USAGE STRATEGY

  Option 1: Quick Start (RECOMMENDED)

  Just copy the Quick Copy-Paste Prompt section and add one reference line:

  [Paste the Quick Copy-Paste Prompt section]

  Also read and follow ALL sections from /workspaces/kcp-worktrees/tmc-planning/TMC-REIMPL-ORCHESTRATOR-PROMPT-TEMPLATE.md

  Show me you understand by:
  1. Listing the 4 grading criteria percentages
  2. Showing current todo list status
  3. Identifying which 8 agents need deployment

  Option 2: Selective Sections

  Pick specific sections based on current needs:

  For starting fresh:
  Read sections "Quick Copy-Paste Prompt" and "Initial Orchestration Setup" from TMC-REIMPL-ORCHESTRATOR-PROMPT-TEMPLATE.md

  Begin by deploying all 8 agents in parallel.

  For fixing violations:
  Read sections "Size Enforcement Template" and "Recovery Procedures" from TMC-REIMPL-ORCHESTRATOR-PROMPT-TEMPLATE.md

  Several agents have >700 line PRs. Execute recovery protocol now.

  Option 3: Full Template (For Complex Situations)

  Read the entire TMC-REIMPL-ORCHESTRATOR-PROMPT-TEMPLATE.md file and follow all protocols.

  Current situation: [describe issues]
  Priority: [what needs immediate attention]
  Begin with: [specific section to execute first]

  ❌ WHAT NOT TO DO

  Don't paste the entire file - It's too long and might dilute focus. The template is designed to be referenced, not pasted in full.

  ✅ BEST PRACTICE

  The most effective approach is:
  1. Paste the Quick Copy-Paste section (it has the core rules)
  2. Add a reference to the full template file
  3. Ask for proof that I understand (grading criteria, todo status, etc.)
  4. Give specific starting instruction (e.g., "Deploy all 8 agents NOW")

  Example Perfect Prompt:

You are the orchestrator-prompt-engineer-task-master for TMC Reimplementation.

MANDATORY RULES (I will grade your performance):
1. Deploy ALL agents in ONE message using parallel Task invocations (I'll check timestamps)
2. EVERY PR must show tmc-pr-line-counter.sh output BEFORE commit (reject if >700 lines)
3. TodoWrite after EVERY agent action (I'll audit the todo list)
4. NO agent proceeds to next PR without review of current PR
5. Any PR >700 lines triggers IMMEDIATE 3-agent split deployment

ACCOUNTABILITY CHECKPOINTS:
□ Show parallel deployment proof (all timestamps within 5 seconds)
□ Show todo list after every 3 agent actions
□ Show line count summary table for all active PRs
□ Show review backlog status
□ Show which agents are active/stopped/blocked

BLOCKING GATES:
- STOP if any agent skips line counting
- STOP if any agent commits >700 lines
- STOP if reviews are more than 2 PRs behind
- STOP if any agent is idle when work exists

  Also follow /workspaces/kcp-worktrees/tmc-planning/TMC-REIMPL-ORCHESTRATOR-PROMPT-TEMPLATE.md from tmc-planning.

  Prove you understand by showing:
  1. The 4 grading criteria with percentages
  2. Current todo list summary
  3. Which agents need what work

  Then immediately deploy the appropriate number of agents for the remaining work in parallel. I'll check timestamps.

  This gives me clear rules, references the full template for details, and demands immediate parallel action with accountability!