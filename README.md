# git-multi - Multi-Repository Git Command Tool

A standalone Go tool for executing git commands across multiple repositories in parallel.

## Quick Setup

### 1. Set environment variable

Add to your `~/.zshrc` or `~/.bashrc`:

```bash
export GMULTI_PATH="your_path"
```

Reload your shell:
```bash
source ~/.zshrc  # or source ~/.bashrc
```

### 2. Build the binary

```bash
cd tools/git-multi
go build -o ~/bin/git-multi main.go
```

### 3. Set up alias (optional)

Add to your `~/.zshrc` or `~/.bashrc`:

```bash
alias gmulti='~/bin/git-multi'
```

### 4. Use it!

```bash
gmulti checkout develop
gmulti pull
gmulti status
```

## Configuration

### Environment Variable

The tool uses the `GMULTI_PATH` environment variable to determine the root directory:

```bash
export GMULTI_PATH="/path/to/your/dir/with/repositories"
```

This should point to a directory containing multiple git repositories.

### Override Path at Runtime

Use the `--path` flag to override the environment variable:
```bash
gmulti --path=/custom/path checkout develop
```

## Usage

```bash
git-multi [options] <git-command> [git-args...]
```

### Options

- `--path=<dir>` - Path to directory with repositories (defaults to `GMULTI_PATH` env var)
- `--exclude=<dirs>` - Comma-separated list of directories to exclude
- `--workers=<n>` - Limit parallel workers (0 = unlimited)
- `--verbose` - Show full git output
- `--fail-fast` - Stop on first failure

### Examples

**Switch all repos to a branch:**
```bash
gmulti checkout FA-279930
```

**Pull all repos:**
```bash
gmulti pull
```

**Check status of all repos:**
```bash
gmulti status
```

**Fetch all repos:**
```bash
gmulti fetch --all
```

**Exclude specific directories:**
```bash
gmulti --exclude="tools,proto" checkout develop
```

**Limit parallelism:**
```bash
gmulti --workers=5 pull
```

**Verbose output:**
```bash
gmulti --verbose status
```

**Use custom path:**
```bash
gmulti --path=/other/repositories/path checkout master
```

## How It Works

1. **Discovery**: Scans the configured directory for all git repositories (directories containing `.git`)
2. **Filtering**: Excludes common directories like `vendor/`, `node_modules/`, `.idea/`, etc.
3. **Parallel Execution**: Runs the git command in all discovered repos concurrently
4. **Results**: Displays colored output with success (✅) and failure (❌) indicators
5. **Summary**: Shows count of succeeded vs failed operations

## Output Format

```
Discovering git repositories...
Found 50 repositories

Executing: git status

✅ bills: On branch master...
✅ profiles: On branch master...
❌ auth: error: pathspec 'feature' did not match...
✅ config: On branch master...
...

────────────────────────────────────────────────────────────────────────────────
Summary: 49 succeeded, 1 failed
```

## Default Exclusions

These directories are automatically excluded:
- `vendor/`
- `node_modules/`
- `.idea/`
- `.vscode/`
- `bin/`
- `build/`
- `dist/`
- Any directory starting with `.`

## Exit Codes

- `0` - All commands succeeded
- `1` - One or more commands failed

## Common Use Cases

### Daily workflow
```bash
# Pull all repos at start of day
gmulti pull

# Check status
gmulti status

# Switch to feature branch
gmulti checkout FA-279930
```

### Branch management
```bash
# See current branches
gmulti branch --show-current

# Create and switch to new branch (if supported)
gmulti checkout -b new-branch
```

### Quick checks
```bash
# Find uncommitted changes
gmulti status --short

# See recent commits
gmulti log -1 --oneline
```

## Troubleshooting

**No repositories found:**
- Check that `GMULTI_PATH` environment variable is set correctly
- Use `--path` flag to specify correct directory
- Verify you have git repositories in the directory

**Path does not exist error:**
- Set `GMULTI_PATH` environment variable: `export GMULTI_PATH="/path/to/repos"`
- Or use `--path` flag with correct directory
- Verify the directory exists

**Some repos fail:**
- This is normal when branches don't exist in all repos
- Use `--verbose` to see detailed errors
- Individual failures don't stop execution (unless `--fail-fast` is used)

## Single File Design

This tool is intentionally designed as a single `main.go` file:
- ✅ Easy to copy and modify
- ✅ No dependencies beyond Go stdlib
- ✅ Can be placed anywhere on your system
- ✅ Simple to customize (just edit the file)
- ✅ Fast compilation

## Customization

To customize for your environment:

1. **Set your root path via environment variable:**
   ```bash
   export GMULTI_PATH="/your/path/to/repositories"
   ```

2. **Add more default exclusions** (edit `main.go`):
   ```go
   var defaultExcludes = []string{
       "vendor",
       "your-custom-dir",
       // ...
   }
   ```

3. **Rebuild if you modified code:**
   ```bash
   go build -o ~/bin/git-multi main.go
   ```

That's it! No additional configuration files needed.