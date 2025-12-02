package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Color codes for output
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
)

// RepoResult holds the result of executing a git command in a repository
type RepoResult struct {
	RepoPath string
	RepoName string
	Success  bool
	Output   string
	Error    error
}

// Config holds the tool configuration
type Config struct {
	Path     string
	Exclude  string
	Workers  int
	Verbose  bool
	FailFast bool
}

// Default directories to exclude
var defaultExcludes = []string{
	"vendor",
	"node_modules",
	".idea",
	".vscode",
	"bin",
	"build",
	"dist",
}

func main() {
	// Parse command-line flags
	config := Config{}
	flag.StringVar(&config.Path, "path", os.Getenv("GMULTI_PATH"), "Path to directory (defaults to GMULTI_PATH env var) with set of repositories")
	flag.StringVar(&config.Exclude, "exclude", "", "Comma-separated list of directories to exclude")
	flag.IntVar(&config.Workers, "workers", 0, "Limit parallel workers (0 = unlimited)")
	flag.BoolVar(&config.Verbose, "verbose", false, "Show full git output")
	flag.BoolVar(&config.FailFast, "fail-fast", false, "Stop on first failure")
	flag.Parse()

	// Get git command and args
	gitArgs := flag.Args()
	if len(gitArgs) == 0 {
		printUsage()
		os.Exit(1)
	}

	cwd := config.Path

	// Verify the path exists
	if _, err := os.Stat(cwd); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "%sError: Directory does not exist: %s%s\n", ColorRed, cwd, ColorReset)
		fmt.Fprintf(os.Stderr, "Use --path flag to specify correct gizmo directory\n")
		os.Exit(1)
	}

	excludes := buildExcludeList(config.Exclude)

	fmt.Printf("%sDiscovering git repositories...%s\n", ColorBlue, ColorReset)
	repos, err := discoverRepos(cwd, excludes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError: Failed to discover repositories: %v%s\n", ColorRed, err, ColorReset)
		os.Exit(1)
	}

	if len(repos) == 0 {
		fmt.Printf("%sNo git repositories found%s\n", ColorYellow, ColorReset)
		os.Exit(0)
	}

	fmt.Printf("%sFound %d repositories%s\n\n", ColorBlue, len(repos), ColorReset)

	fmt.Printf("%sExecuting: git %s%s\n\n", ColorBlue, strings.Join(gitArgs, " "), ColorReset)
	results := executeInParallel(repos, gitArgs, config)

	successCount, failCount := formatOutput(results, config.Verbose)

	fmt.Printf("\n%s", strings.Repeat("─", 80))
	fmt.Printf("\n%sSummary: ", ColorBlue)
	if successCount > 0 {
		fmt.Printf("%s%d succeeded%s", ColorGreen, successCount, ColorReset)
	}
	if failCount > 0 {
		if successCount > 0 {
			fmt.Printf(", ")
		}
		fmt.Printf("%s%d failed%s", ColorRed, failCount, ColorReset)
	}
	fmt.Printf("%s\n", ColorReset)

	if failCount > 0 {
		os.Exit(1)
	}
}

// buildExcludeList builds the final exclusion list from config and defaults
func buildExcludeList(configExclude string) map[string]bool {
	excludes := make(map[string]bool)

	// Add default excludes
	for _, dir := range defaultExcludes {
		excludes[dir] = true
	}

	// Add config excludes
	if configExclude != "" {
		for _, dir := range strings.Split(configExclude, ",") {
			excludes[strings.TrimSpace(dir)] = true
		}
	}

	return excludes
}

// discoverRepos finds all git repositories in the given directory
func discoverRepos(rootDir string, excludes map[string]bool) ([]string, error) {
	var repos []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if not a directory
		if !info.IsDir() {
			return nil
		}

		// Get relative path for exclusion check
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Check if directory should be excluded
		dirName := info.Name()
		if excludes[dirName] || strings.HasPrefix(dirName, ".") {
			return filepath.SkipDir
		}

		// Check exclusion by relative path
		for exclude := range excludes {
			if strings.Contains(relPath, exclude) {
				return filepath.SkipDir
			}
		}

		// Check if this is a git repository
		gitDir := filepath.Join(path, ".git")
		if _, err = os.Stat(gitDir); err == nil {
			repos = append(repos, path)
			// Don't descend into nested repos
			return filepath.SkipDir
		}

		return nil
	})

	return repos, err
}

// executeInParallel executes the git command in all repositories concurrently
func executeInParallel(repos []string, gitArgs []string, config Config) []RepoResult {
	results := make([]RepoResult, len(repos))
	var wg sync.WaitGroup

	// Create channel for work distribution if workers limit is set
	if config.Workers > 0 {
		semaphore := make(chan struct{}, config.Workers)
		for i, repo := range repos {
			wg.Add(1)
			go func(idx int, repoPath string) {
				defer wg.Done()
				semaphore <- struct{}{} // Acquire
				results[idx] = executeGitCommand(repoPath, gitArgs)
				<-semaphore // Release

				// Check fail-fast
				if config.FailFast && !results[idx].Success {
					fmt.Fprintf(os.Stderr, "%s\nFail-fast enabled, stopping execution%s\n", ColorRed, ColorReset)
					os.Exit(1)
				}
			}(i, repo)
		}
	} else {
		// Unlimited parallelism
		for i, repo := range repos {
			wg.Add(1)
			go func(idx int, repoPath string) {
				defer wg.Done()
				results[idx] = executeGitCommand(repoPath, gitArgs)

				// Check fail-fast
				if config.FailFast && !results[idx].Success {
					fmt.Fprintf(os.Stderr, "%s\nFail-fast enabled, stopping execution%s\n", ColorRed, ColorReset)
					os.Exit(1)
				}
			}(i, repo)
		}
	}

	wg.Wait()
	return results
}

// executeGitCommand executes a git command in the specified repository
func executeGitCommand(repoPath string, gitArgs []string) RepoResult {
	result := RepoResult{
		RepoPath: repoPath,
		RepoName: filepath.Base(repoPath),
	}

	cmd := exec.Command("git", gitArgs...)
	cmd.Dir = repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine stdout and stderr
	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	result.Output = strings.TrimSpace(output)
	result.Success = err == nil
	result.Error = err

	return result
}

// formatOutput formats and displays the results
func formatOutput(results []RepoResult, verbose bool) (successCount, failCount int) {
	for _, result := range results {
		if result.Success {
			successCount++
			if verbose {
				fmt.Printf("%s✅ %s:%s\n", ColorGreen, result.RepoName, ColorReset)
				if result.Output != "" {
					printIndented(result.Output)
				}
			} else {
				// Compact output
				fmt.Printf("%s✅ %s:%s %s\n", ColorGreen, result.RepoName, ColorReset, result.Output)
			}
		} else {
			failCount++
			fmt.Printf("%s❌ %s:%s\n", ColorRed, result.RepoName, ColorReset)
			if result.Output != "" {
				printIndented(result.Output)
			}
			if result.Error != nil && verbose {
				printIndented(fmt.Sprintf("Error: %v", result.Error))
			}
		}
	}

	return successCount, failCount
}

// printIndented prints text with indentation
func printIndented(text string) {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Printf("%s   %s%s\n", ColorGray, line, ColorReset)
		}
	}
}

// printUsage prints usage information
func printUsage() {
	fmt.Printf("%sgit-multi - Execute git commands across multiple repositories%s\n\n", ColorBlue, ColorReset)
	fmt.Printf("Usage: git-multi [options] <git-command> [git-args...]\n\n")
	fmt.Printf("Options:\n")
	flag.PrintDefaults()
	fmt.Printf("\nExamples:\n")
	fmt.Printf("  git-multi checkout develop\n")
	fmt.Printf("  git-multi pull\n")
	fmt.Printf("  git-multi status\n")
	fmt.Printf("  git-multi fetch --all\n")
	fmt.Printf("  git-multi --path=/custom/path/to/gizmo checkout develop\n")
	fmt.Printf("  git-multi --exclude=\"tools,game_proto\" checkout FA-279930\n")
	fmt.Printf("  git-multi --workers=5 pull\n")
	fmt.Printf("  git-multi --verbose status\n")
	fmt.Printf("\nSetup as alias:\n")
	fmt.Printf("  1. Build: go build -o ~/bin/git-multi main.go\n")
	fmt.Printf("  2. Add to ~/.zshrc or ~/.bashrc:\n")
	fmt.Printf("     alias gmulti='~/bin/git-multi'\n")
	fmt.Printf("  3. Use: gmulti checkout develop\n")
}
