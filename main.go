package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/teilomillet/gollm"
)

type request struct {
	startingTime   time.Time
	path           string
	branchName     string
	commitTemplate string
}

func main() {
	r := &request{}
	r.init()

	llm, err := gollm.NewLLM(
		gollm.SetProvider("ollama"),
		gollm.SetModel("llama3.2"),
		gollm.SetDebugLevel(gollm.LogLevelWarn),
	)
	if err != nil {
		log.Fatalf("Failed to create LLM: %v", err)
	}

	ctx := context.Background()

	out, err := exec.Command("git", "diff").Output()
	if err != nil {
		log.Fatalf("Failed to run git diff: %v", err)
	}

	branchNameBytes, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err == nil {
		r.branchName = string(branchNameBytes)
	}

	_ = r.setCommitTemplate()

	rawPrompt := "# Instructions\n\n1. Given a git diff output write a commit message for the changes.\n2. Follow the commit template\n3. Craft a title based on the content of the diff\n4. For the ticket section extarct it from the branch name, just the first 2 sections of the branch name.\n5. Your response should be following the template.\n\n# Git diff\n" + string(out) + "\n\n# Branch name\n" + r.branchName + "\n\n# Commit template\n" + r.commitTemplate + "\n---\nResponse:"

	r.writeToFile(rawPrompt)

	prompt := gollm.NewPrompt(rawPrompt)
	response, err := llm.Generate(ctx, prompt)
	if err != nil {
		log.Fatalf("Failed to generate text: %v", err)
	}
	fmt.Printf("Response: %s\n", response)
}

func (r *request) init() {
	r.startingTime = time.Now()
	r.path = filepath.Join(".logs", r.startingTime.Format("2006-01-02T15:04:05")+".log")
	r.branchName = "main"
	r.commitTemplate = `<type>[optional scope]: <subject>

What: <explain what changed here>
Why: <explain why was it changed>

<ticket>`

	err := os.Mkdir(".logs", os.ModePerm)
	if err != nil && !errors.Is(err, os.ErrExist) {
		fmt.Printf("Failed to create logs directory: %v", err)
	}
}

func (r *request) writeToFile(content string) {
	f, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		fmt.Printf("Failed to write to log file: %v", err)
	}
}

func (r *request) setCommitTemplate() error {
	commitTemplatePath, err := exec.Command("git", "config", "commit.template").Output()
	if err != nil {
		return fmt.Errorf("Failed to run git config: %v", err)
	}

	commitTemplatePath = bytes.TrimSpace(commitTemplatePath)

	fp, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Failed to get working directory: %v", err)
	}

	commitTemplateBytes, err := os.ReadFile(filepath.Clean((filepath.Join(fp, string(commitTemplatePath)))))
	if err == nil {
		r.commitTemplate = string(commitTemplateBytes)
	}

	return nil
}
