---
description: Run tests across all services
---

# Test All Services

Runs the complete test suite across all North Cloud services in parallel.

## Usage

This command will:
1. Navigate to the project root
2. Run tests for all services using Task runner
3. Execute tests in parallel for speed
4. Report results for each service

## Command

```bash
cd /home/jones/dev/north-cloud && task test
```

## Services Tested

- Crawler (18 tests + benchmarks)
- Source Manager (7 test suites)
- Classifier (6 classification tests)
- Publisher (routing and filtering tests)
- Index Manager (index operations)
- Search (search and faceting)
- Auth (JWT and authentication)

## Output

Shows pass/fail status for each service with:
- Number of tests run
- Execution time
- Any failures or errors

## When to Use

- Before committing changes
- Before creating pull requests
- After major refactoring
- During CI/CD pipeline

## Alternative

For sequential testing (easier to debug):
```bash
cd /home/jones/dev/north-cloud && task test:sequential
```
