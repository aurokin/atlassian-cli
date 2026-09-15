package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func extensionFixture(t *testing.T) []byte {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	program := `package main
import("encoding/json";"os";"strconv")
func main(){ path,_:=os.Executable(); json.NewEncoder(os.Stdout).Encode(struct{Args []string;Path string}{os.Args[1:],path});if code,_:=strconv.Atoi(os.Getenv("ATL_EXTENSION_TEST_EXIT"));code!=0{os.Stderr.WriteString("fixture stderr\n");os.Exit(code)}}`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "fixture")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", output, source)
	if data, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build extension fixture: %v\n%s", err, data)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func processEnv(p process, key, value string) process {
	env := make([]string, 0, len(p.env)+1)
	for _, entry := range p.env {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(name, key) {
			env = append(env, entry)
		}
	}
	p.env = append(env, key+"="+value)
	return p
}

func writeExtension(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

type extensionOutput struct {
	Args []string
	Path string
}

func TestExtensionProcesses(t *testing.T) {
	data := extensionFixture(t)
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			first, second := t.TempDir(), t.TempDir()
			p = processEnv(p, "PATH", first+string(os.PathListSeparator)+second)
			suffix := ""
			if runtime.GOOS == "windows" {
				suffix = ".exe"
				p = processEnv(p, "PATHEXT", ".EXE;.COM")
			}
			prefix := "atl-" + product + "-"
			chosen := writeExtension(t, first, prefix+"echo"+suffix, data)
			writeExtension(t, second, prefix+"echo"+suffix, data)
			writeExtension(t, first, "unrelated"+suffix, data)
			writeExtension(t, first, "atl-other-echo"+suffix, data)
			blocked := writeExtension(t, first, prefix+"not-executable", data)
			if runtime.GOOS != "windows" {
				if err := os.Chmod(blocked, 0600); err != nil {
					t.Fatal(err)
				}
			}
			var entries []struct{ Name, Executable string }
			listed := p.run(t, product, "extension", "list", "--json")
			success(t, listed)
			if err := json.Unmarshal([]byte(listed.stdout), &entries); err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name != "echo" || entries[0].Executable != chosen {
				t.Fatalf("discovery differs from PATH precedence: %+v", entries)
			}
			args := []string{"space value", "--unknown-extension-flag", "quote\"value", "", "--", "tail"}
			for _, invocation := range [][]string{{"extension", "exec", "echo"}, {"echo"}} {
				result := p.run(t, product, append(append([]string{}, invocation...), args...)...)
				success(t, result)
				var out extensionOutput
				if err := json.Unmarshal([]byte(result.stdout), &out); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(out.Args, args) || !samePath(out.Path, chosen) {
					t.Fatalf("extension argv or selected binary differs: %+v", out)
				}
				failing := processEnv(p, "ATL_EXTENSION_TEST_EXIT", "23").run(t, product, append(append([]string{}, invocation...), args...)...)
				if failing.exit != 23 || failing.stderr != "fixture stderr\n" {
					t.Fatalf("extension exit/stderr not propagated: %+v", failing)
				}
			}
			if runtime.GOOS != "windows" {
				link := filepath.Join(first, prefix+"linked")
				if err := os.Symlink(chosen, link); err != nil {
					t.Fatal(err)
				}
				result := p.run(t, product, "extension", "exec", "linked", "symlink")
				success(t, result)
				listed = p.run(t, product, "extension", "list", "--json")
				success(t, listed)
				if err := json.Unmarshal([]byte(listed.stdout), &entries); err != nil {
					t.Fatal(err)
				}
				if len(entries) != 2 || entries[1].Name != "linked" || entries[1].Executable != link {
					t.Fatalf("symlink missing from discovery: %+v", entries)
				}
			}
		})
	}
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func TestWindowsExtensionResolution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Log("native Windows resolution is covered by the Windows CI job")
		return
	} // Native Windows CI owns this platform contract.
	data := extensionFixture(t)
	for _, product := range []string{"jira", "conf", "bb"} {
		t.Run(product, func(t *testing.T) {
			p := isolated(t)
			dir := t.TempDir()
			p = processEnv(p, "PATH", dir)
			prefix := "atl-" + product + "-"
			com := writeExtension(t, dir, strings.ToUpper(prefix)+"MiXeD.COM", data)
			exe := writeExtension(t, dir, prefix+"mixed.exe", data)
			for _, choice := range []struct{ pathext, chosen string }{{".COM;.EXE", com}, {".EXE;.COM", exe}} {
				selected := processEnv(p, "PATHEXT", choice.pathext)
				var entries []struct{ Name, Executable string }
				listed := selected.run(t, product, "extension", "list", "--json")
				success(t, listed)
				if err := json.Unmarshal([]byte(listed.stdout), &entries); err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 || !strings.EqualFold(entries[0].Name, "mixed") || !samePath(entries[0].Executable, choice.chosen) {
					t.Fatalf("discovery must agree with Windows case/PATHEXT %s: %+v", choice.pathext, entries)
				}
				for _, invocation := range [][]string{{"extension", "exec", "mixed"}, {"MiXeD"}} {
					result := selected.run(t, product, invocation...)
					success(t, result)
					var out extensionOutput
					if err := json.Unmarshal([]byte(result.stdout), &out); err != nil {
						t.Fatal(err)
					}
					if !samePath(out.Path, choice.chosen) {
						t.Fatalf("PATHEXT chose %s, want %s", out.Path, choice.chosen)
					}
				}
			}
		})
	}
}

// Generated callbacks are executed against real binaries and isolated profile
// configuration. Bash/Zsh inject cursor state and capture the shell's candidate
// boundary; Fish and PowerShell use their noninteractive completion APIs.
func TestShellCompletionProcesses(t *testing.T) {
	required := map[string]bool{}
	selected := os.Getenv("ATL_E2E_REQUIRED_SHELLS")
	if selected == "" {
		selected = "bash"
		if runtime.GOOS == "windows" {
			selected = "powershell"
		}
	}
	for _, name := range strings.Split(selected, ",") {
		required[strings.TrimSpace(name)] = true
	}
	for name := range required {
		if name != "bash" && name != "zsh" && name != "fish" && name != "powershell" {
			t.Fatalf("unknown required shell %q", name)
		}
	}
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		executable, err := exec.LookPath(shell)
		if shell == "powershell" {
			executable, err = exec.LookPath("pwsh")
			if err != nil {
				executable, err = exec.LookPath("powershell")
			}
		}
		if err != nil && required[shell] {
			t.Fatalf("required completion shell %s unavailable: %v", shell, err)
		}
		for _, product := range []string{"jira", "conf", "bb"} {
			t.Run(shell+"/"+product, func(t *testing.T) {
				p := isolated(t)
				p.profile(t, product, "http://127.0.0.1:1")
				generated := p.run(t, product, "completion", shell)
				success(t, generated)
				if !strings.Contains(generated.stdout, "atl-"+product) {
					t.Fatal("completion omitted binary identity")
				}
				if err != nil {
					t.Logf("generated %s completion; native execution excluded because shell is not installed (require it with ATL_E2E_REQUIRED_SHELLS)", shell)
					return
				}
				script := filepath.Join(p.home, "completion-script")
				if shell == "powershell" {
					script += ".ps1"
				}
				if err := os.WriteFile(script, []byte(generated.stdout), 0600); err != nil {
					t.Fatal(err)
				}
				shellPath := filepath.Dir(binaries[product]) + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
				if runtime.GOOS == "windows" {
					shellPath = filepath.Dir(binaries[product]) + string(os.PathListSeparator) + filepath.Join(os.Getenv("SystemRoot"), "System32")
					// PowerShell relies on PATHEXT when resolving the bare CLI
					// name; the isolated fixture does not inherit user values.
					p = processEnv(p, "PATHEXT", ".COM;.EXE;.BAT;.CMD")
				}
				p = processEnv(p, "PATH", shellPath)
				p = processEnv(p, "ATL_COMPLETION_SCRIPT", script)
				p = processEnv(p, "ATL_COMPLETION_BINARY", "atl-"+product)
				cases := []struct {
					line string
					want []string
				}{{"auth login --token-style cloud", []string{"cloud-classic", "cloud-scoped"}}, {"status --site fi", []string{"fixture"}}}
				if product == "conf" {
					cases = append(cases, struct {
						line string
						want []string
					}{"page create --body-format sto", []string{"storage"}})
				}
				for _, tc := range cases {
					invocation := processEnv(p, "ATL_COMPLETION_LINE", "atl-"+product+" "+tc.line)
					output := runCompletionShell(t, invocation, executable, shell)

					candidates := map[string]bool{}
					for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
						candidate, _, _ := strings.Cut(strings.TrimSpace(line), "\t")
						candidate, _, _ = strings.Cut(candidate, ":")
						if candidate != "" {
							candidates[candidate] = true
						}
					}
					for _, want := range tc.want {
						if !candidates[want] {
							t.Fatalf("%s completion of %q missing %q: %s", shell, tc.line, want, output)
						}
					}
					// Zsh's _describe performs final prefix matching interactively;
					// this callback fixture captures its input candidates instead.
					if shell != "zsh" && len(candidates) != len(tc.want) {
						t.Fatalf("completion returned unexpected candidates: %s", output)
					}

				}
			})
		}
	}
}

func runCompletionShell(t *testing.T, p process, executable, shell string) string {
	t.Helper()
	var script string
	var args []string
	switch shell {
	case "bash":
		script = `source "$ATL_COMPLETION_SCRIPT"
_init_completion() { read -r -a words <<< "$ATL_COMPLETION_LINE"; cword=$((${#words[@]}-1)); cur=${words[cword]}; prev=${words[cword-1]}; }
compopt() { :; }
"__start_${ATL_COMPLETION_BINARY}"
printf '%s\n' "${COMPREPLY[@]}"
`
		args = []string{"--noprofile", "--norc", "-c", script}
	case "zsh":
		script = `compdef() { :; }
_describe() { print -rl -- "${completions[@]}"; }
source "$ATL_COMPLETION_SCRIPT"
words=( ${(z)ATL_COMPLETION_LINE} )
CURRENT=${#words[@]}
"_${ATL_COMPLETION_BINARY}"
`
		args = []string{"-f", "-c", script}
	case "fish":
		script = `source "$ATL_COMPLETION_SCRIPT"
complete --do-complete "$ATL_COMPLETION_LINE"
`
		args = []string{"--no-config", "-c", script}
	case "powershell":
		script = `$ErrorActionPreference = 'Stop'
. $env:ATL_COMPLETION_SCRIPT
Get-Command $env:ATL_COMPLETION_BINARY -CommandType Application -ErrorAction Stop | Out-Null
[System.Management.Automation.CommandCompletion]::CompleteInput($env:ATL_COMPLETION_LINE, $env:ATL_COMPLETION_LINE.Length, $null).CompletionMatches | ForEach-Object { $_.CompletionText }
`
		args = []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script}
	default:
		t.Fatalf("unsupported shell %s", shell)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = p.env
	cmd.Dir = p.home
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s completion failed: %v\nstdout=%s\nstderr=%s", shell, err, stdout.String(), stderr.String())
	}
	diagnostics := strings.TrimSpace(stderr.String())
	// Cobra deliberately prints this diagnostic from __complete. PowerShell's
	// native completer can forward it even though its generated script redirects
	// the invocation. Accept only this documented successful directive.
	if shell == "powershell" && diagnostics == "Completion ended with directive: ShellCompDirectiveNoFileComp" {
		diagnostics = ""
	}
	if diagnostics != "" {
		t.Fatalf("%s completion stderr: %s", shell, stderr.String())
	}
	return stdout.String()
}
