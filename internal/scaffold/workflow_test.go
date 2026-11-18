package scaffold_test

import (
	"fmt"
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

func TestWorkflowHasTheFourTriggers(t *testing.T) {
	doc := workflowDoc(t)

	on, ok := doc["on"]
	if !ok {
		t.Fatal("the workflow has no on block")
	}

	triggers := keys(t, on)
	want := map[string]bool{"pull_request": false, "push": false, "schedule": false, "workflow_dispatch": false}
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

	branches, _ := child(t, on, "push").(map[string]any)["branches"].([]any)
	if len(branches) != 1 || branches[0] != "main" {
		t.Errorf("push branches = %v, want main only", branches)
	}

	cron := child(t, on, "schedule").([]any)[0].(map[string]any)["cron"]
	if cron != "0 3 * * 1" {
		t.Errorf("cron = %v, want a weekly run at 03:00 on Monday", cron)
	}
}

func TestWorkflowHasThreeJobs(t *testing.T) {
	doc := workflowDoc(t)

	got := keys(t, child(t, doc, "jobs"))
	if len(got) != 3 {
		t.Fatalf("jobs = %v, want validate, plan, and apply", got)
	}
	for _, job := range []string{"validate", "plan", "apply"} {
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
	for _, job := range []string{"validate", "plan", "apply"} {
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

	for _, job := range []string{"validate", "plan", "apply"} {
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

// A merge is the approval, so the merge event reaches the job that holds the
// shares. A pull request never does.
func TestApplyJobRunsOnAMerge(t *testing.T) {
	doc := workflowDoc(t)

	condition, _ := child(t, doc, "jobs", "apply").(map[string]any)["if"].(string)
	for _, event := range []string{"'push'", "'schedule'", "'workflow_dispatch'"} {
		if !strings.Contains(condition, event) {
			t.Errorf("the apply condition does not name %s: %q", event, condition)
		}
	}
	if strings.Contains(condition, "pull_request") {
		t.Errorf("the apply condition names a pull request, so a fork could reach the shares: %q", condition)
	}

	run, _ := stepNamed(t, doc, "apply", "Apply")["run"].(string)
	if !strings.Contains(run, "meshy apply --json") {
		t.Errorf("the apply step runs %q", run)
	}
}

func TestApplyJobUnlocksFromTheEnvironment(t *testing.T) {
	doc := workflowDoc(t)

	step := stepNamed(t, doc, "apply", "Unlock")
	env, _ := step["env"].(map[string]any)
	for i := 1; i <= 3; i++ {
		name := fmt.Sprintf("MESHY_SHARE_%d", i)
		if env[name] != "${{ secrets."+name+" }}" {
			t.Errorf("env %s = %v, want the secret %s", name, env[name], name)
		}
	}

	run, _ := step["run"].(string)
	if !strings.Contains(run, "meshy unlock --from-env") {
		t.Error("the unlock step does not read the shares from the environment")
	}
	if !strings.Contains(run, `echo "::error::`) || !strings.Contains(run, "exit 1") {
		t.Error("the unlock step does not name a missing share as an error")
	}
}

// The weekly run reads the plan and applies only when the plan holds a change,
// which covers drift and a renewal. A merge applies without a second gate.
func TestApplyJobAppliesOnlyWhenThePlanChanges(t *testing.T) {
	doc := workflowDoc(t)

	plan := stepNamed(t, doc, "apply", "Plan")
	if plan["id"] != "plan" {
		t.Fatalf("the plan step has id %v, want plan", plan["id"])
	}

	run, _ := plan["run"].(string)
	for _, want := range []string{
		"meshy plan --json",
		"$GITHUB_STEP_SUMMARY",
		`"$MESHY_EVENT" = "push"`,
		`"$code" -eq 1`,
		`>> "$GITHUB_OUTPUT"`,
	} {
		if !strings.Contains(run, want) {
			t.Errorf("the plan step of the apply job does not hold %q", want)
		}
	}
	if strings.Contains(run, "jq") {
		t.Error("the plan step reads the change count with jq, but the exit code already carries it")
	}

	for _, name := range []string{"Apply", "Unlock"} {
		condition, _ := stepNamed(t, doc, "apply", name)["if"].(string)
		if condition != "steps.plan.outputs.apply == 'true'" {
			t.Errorf("the %s step is conditional on %q", name, condition)
		}
	}
}

// The vault closes and the key goes even when the apply step failed.
func TestApplyJobClosesTheVaultAndRemovesTheKey(t *testing.T) {
	doc := workflowDoc(t)

	lock := stepNamed(t, doc, "apply", "Lock")
	if lock["if"] != "always()" {
		t.Errorf("the lock step runs when %v, want always()", lock["if"])
	}
	if lock["run"] != "meshy lock" {
		t.Errorf("the lock step runs %v", lock["run"])
	}

	key := stepNamed(t, doc, "apply", "Remove the ssh identity")
	if key["if"] != "always()" {
		t.Errorf("the cleanup step runs when %v, want always()", key["if"])
	}

	order := steps(t, doc, "apply")
	at := func(name string) int {
		for i, step := range order {
			if step["name"] == name {
				return i
			}
		}
		t.Fatalf("job apply has no step named %q", name)
		return -1
	}
	if at("Lock") < at("Unlock") || at("Lock") < at("Apply") {
		t.Error("the lock step runs before the vault opens or the apply runs")
	}
}

// apply signs and renews, so the job needs the pinned nebula-cert in the cache
// that ca.Find reads. The archive and the member are checked before the tool is
// used.
func TestApplyJobInstallsTheSigningTool(t *testing.T) {
	doc := workflowDoc(t)

	step := stepNamed(t, doc, "apply", "Install nebula-cert")
	if step["if"] != "steps.plan.outputs.apply == 'true'" {
		t.Errorf("the install step is conditional on %q, so a quiet week downloads a release", step["if"])
	}

	run, _ := step["run"].(string)
	for _, want := range []string{
		"SHASUM256.txt",
		"sha256sum -c -",
		`"$asset/nebula-cert"`,
		".meshy/bin/$version/linux-amd64/nebula-cert",
	} {
		if !strings.Contains(run, want) {
			t.Errorf("the install step does not hold %q", want)
		}
	}
}

func TestApplyJobWritesTheStepSummary(t *testing.T) {
	doc := workflowDoc(t)

	run, _ := stepNamed(t, doc, "apply", "Apply")["run"].(string)
	if !strings.Contains(run, "$GITHUB_STEP_SUMMARY") {
		t.Error("the apply step does not write the step summary")
	}
	if !strings.Contains(run, `cat "$RUNNER_TEMP/apply.json"`) {
		t.Error("the apply step does not put the report in the summary")
	}
}

// The plan job reports a difference and holds no share, so a run that reaches
// it never opens the vault.
func TestPlanJobHoldsNoShare(t *testing.T) {
	doc := workflowDoc(t)

	for _, step := range steps(t, doc, "plan") {
		run, _ := step["run"].(string)
		for _, forbidden := range []string{"MESHY_SHARE", "meshy unlock", "meshy lock", "meshy apply"} {
			if strings.Contains(run, forbidden) {
				t.Errorf("step %q of plan holds %q", step["name"], forbidden)
			}
		}
	}
}
