package scaffold_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/zinzan-vdm/cli-meshy/internal/scaffold"
)

// workflowYAML scaffolds a mesh and returns the workflow file it wrote.
func workflowYAML(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := scaffold.Init(scaffold.Options{Dir: dir, Name: "homelab"}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	path := filepath.Join(dir, ".github", "workflows", "meshy.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the workflow: %v", err)
	}
	return string(body)
}

// workflowDoc parses the workflow into nested maps. yaml.v3 reads the key
// "on" as a boolean in some documents, so the lookup accepts both keys.
func workflowDoc(t *testing.T) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(workflowYAML(t)), &doc); err != nil {
		t.Fatalf("the workflow is not valid YAML: %v", err)
	}
	return doc
}

func child(t *testing.T, node any, keys ...string) any {
	t.Helper()
	for _, key := range keys {
		m, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("no map at %q", key)
		}
		next, ok := m[key]
		if !ok {
			t.Fatalf("no key %q", key)
		}
		node = next
	}
	return node
}

func keys(t *testing.T, node any) []string {
	t.Helper()
	m, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("not a map")
	}
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

// steps returns the steps of a job in order.
func steps(t *testing.T, doc map[string]any, job string) []map[string]any {
	t.Helper()
	raw, ok := child(t, doc, "jobs", job, "steps").([]any)
	if !ok {
		t.Fatalf("job %q has no steps", job)
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		step, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("a step of %q is not a map", job)
		}
		out = append(out, step)
	}
	return out
}

func stepNamed(t *testing.T, doc map[string]any, job, name string) map[string]any {
	t.Helper()
	for _, step := range steps(t, doc, job) {
		if step["name"] == name {
			return step
		}
	}
	t.Fatalf("job %q has no step named %q", job, name)
	return nil
}

func TestWorkflowHasTheThreeTriggers(t *testing.T) {
	doc := workflowDoc(t)

	on, ok := doc["on"]
	if !ok {
		t.Fatal("the workflow has no on block")
	}

	triggers := keys(t, on)
	want := map[string]bool{"pull_request": false, "schedule": false, "workflow_dispatch": false}
	for _, trigger := range triggers {
		if _, ok := want[trigger]; !ok {
			t.Errorf("unexpected trigger %q", trigger)
		}
		want[trigger] = true
	}
	for trigger, seen := range want {
		if !seen {
			t.Errorf("trigger %q is missing", trigger)
		}
	}

	cron := child(t, on, "schedule").([]any)[0].(map[string]any)["cron"]
	if cron != "0 3 * * 1" {
		t.Errorf("cron = %v, want a weekly run at 03:00 on Monday", cron)
	}
}

func TestWorkflowHasTwoJobs(t *testing.T) {
	doc := workflowDoc(t)

	got := keys(t, child(t, doc, "jobs"))
	if len(got) != 2 {
		t.Fatalf("jobs = %v, want validate and plan", got)
	}
	for _, job := range []string{"validate", "plan"} {
		if _, ok := child(t, doc, "jobs").(map[string]any)[job]; !ok {
			t.Fatalf("job %q is missing, got %v", job, got)
		}
	}
}

func TestValidateJobRunsOnEveryEvent(t *testing.T) {
	doc := workflowDoc(t)

	if condition, ok := child(t, doc, "jobs", "validate").(map[string]any)["if"]; ok {
		t.Errorf("the validate job is conditional on %v, so a fork never validates", condition)
	}
	if job := stepNamed(t, doc, "validate", "Validate"); job["run"] != "meshy validate" {
		t.Errorf("the validate step runs %v", job["run"])
	}
}

func TestPlanJobSkipsAFork(t *testing.T) {
	doc := workflowDoc(t)

	condition, _ := child(t, doc, "jobs", "plan").(map[string]any)["if"].(string)
	if !strings.Contains(condition, "github.event.pull_request.head.repo.full_name == github.repository") {
		t.Errorf("the plan condition does not compare the fork with this repository: %q", condition)
	}
	if !strings.Contains(condition, "'schedule'") {
		t.Errorf("the plan condition skips the weekly run: %q", condition)
	}
}

func TestPlanJobWritesTheStepSummary(t *testing.T) {
	doc := workflowDoc(t)

	plan := stepNamed(t, doc, "plan", "Plan")
	run, _ := plan["run"].(string)
	if !strings.Contains(run, "$GITHUB_STEP_SUMMARY") {
		t.Error("the plan step does not write the step summary")
	}
	if !strings.Contains(run, "cat \"$RUNNER_TEMP/plan.json\"") {
		t.Error("the plan step does not put the JSON in the summary")
	}
	if !strings.Contains(run, "meshy plan --json") {
		t.Error("the plan step does not run meshy plan --json")
	}
}

// A difference is the report the reviewer approves, so exit 1 must pass the
// job. Only exit 2, which means no plan was produced, fails it.
func TestPlanJobFailsOnlyWithoutAPlan(t *testing.T) {
	doc := workflowDoc(t)

	run, _ := stepNamed(t, doc, "plan", "Plan")["run"].(string)
	fail := regexp.MustCompile(`if \[ "\$code" -ge 2 \]; then`)
	if !fail.MatchString(run) {
		t.Error("the plan step does not fail on exit code 2")
	}
	if strings.Contains(run, `exit "$code"`) || strings.Contains(run, "exit $code") {
		t.Error("the plan step returns the exit code of meshy plan, so a difference fails the job")
	}
}

func TestPlanJobNeedsBothSecrets(t *testing.T) {
	doc := workflowDoc(t)

	step := stepNamed(t, doc, "plan", "Write the ssh identity")
	env, _ := step["env"].(map[string]any)
	for secret, name := range map[string]string{
		"MESHY_KNOWN_HOSTS":     "HOSTS",
		"MESHY_SSH_PRIVATE_KEY": "KEY",
	} {
		if env[name] != "${{ secrets."+secret+" }}" {
			t.Errorf("env %s = %v, want the secret %s", name, env[name], secret)
		}
	}

	run, _ := step["run"].(string)
	if !strings.Contains(run, `echo "::error::`) {
		t.Error("the step does not name the missing secret as an error")
	}
	if !strings.Contains(run, "exit 1") {
		t.Error("the step does not fail when a secret is absent")
	}
	if !strings.Contains(run, `umask 077`) {
		t.Error("the key file is not private")
	}
	for _, name := range []string{"MESHY_KNOWN_HOSTS", "MESHY_SSH_KEY"} {
		if _, ok := env[name]; !ok {
			t.Errorf("the plan does not pass %s to meshy", name)
		}
	}
}

func TestWorkflowPinsEveryAction(t *testing.T) {
	doc := workflowDoc(t)

	pinned := regexp.MustCompile(`^[^@]+@[0-9a-f]{40}$`)
	for _, job := range []string{"validate", "plan"} {
		for _, step := range steps(t, doc, job) {
			uses, ok := step["uses"].(string)
			if !ok {
				continue
			}
			if !pinned.MatchString(uses) {
				t.Errorf("action %q in job %q is not pinned by a commit SHA", uses, job)
			}
		}
	}
}

func TestWorkflowKeepsTheSecretsOutOfTheLog(t *testing.T) {
	doc := workflowDoc(t)

	for _, job := range []string{"validate", "plan"} {
		for _, step := range steps(t, doc, job) {
			run, _ := step["run"].(string)
			if strings.Contains(run, "secrets.") {
				t.Errorf("step %q of %q reads a secret in a run block, which can print it", step["name"], job)
			}
			if strings.Contains(run, "set -x") {
				t.Errorf("step %q of %q traces its commands", step["name"], job)
			}
		}
	}
}

func TestWorkflowAppliesNothing(t *testing.T) {
	body := workflowYAML(t)

	for _, forbidden := range []string{"meshy apply", "meshy unlock", "meshy lock", "secrets.age"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the workflow runs %q, but only a merge applies the mesh", forbidden)
		}
	}
}
