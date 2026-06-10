# Demo runsheet — `processImage` (boundary adapters)

One-example show-and-tell: listmonk's `processImage`, monolith → lifted service.
Nothing here gets typed live except short, pre-tested lines. **Laptop = editor + static
artifacts only. Buildserver = anything that compiles Go or touches kind/docker.**

> Buildserver = CloudLab `monolift-buildserver`, repo at `/local/repository`
> (adjust the ssh alias if yours differs). Laptop repo: `/Users/tgoodwin/projects/monolift`.

---

## Fill these in during pre-warm (so you reference, not discover, live)

```
RUN ID            : ____________________
LIFTED NAMESPACE  : mlv2-lifted-activation-listmonk-processimage-____________________
HOST DEPLOY       : ____________________   (kubectl -n $NS get deploy)
EXTRACTED DEPLOY  : ____________________   (likely monolift-extracted-processimage)
```

---

## T‑minus (before the talk, on the BUILDSERVER)

Do the slow/fragile things now; leave the cluster up.

```bash
ssh monolift-buildserver           # your CloudLab alias for the build node
cd /local/repository

# 0. SYNC — all demo code is committed on origin/sprint-51; only the corpus is fetched.
git fetch origin && git checkout sprint-51 && git pull --ff-only
#    corpus (evaluation/listmonk is gitignored): idempotent pinned-SHA clone, safe to re-run
./docs/research/runs/SPRINT-0034-lift-utility-corpus/clone-evaluation-targets.sh listmonk
#    corpus is cloned root-owned & READ-ONLY for you — take ownership so Beats 3/4 can edit media.go
#    (also clears git's "dubious ownership"; passwordless sudo works on the buildbox):
sudo chown -R "$(id -un):$(id -gn)" evaluation/listmonk
git -C evaluation/listmonk rev-parse HEAD     # want 3f4917035f63a82c93e19dedee8a48e55e291974
git -C evaluation/listmonk status -s          # want clean/empty
test -f evaluation/listmonk/cmd/media.go && echo "corpus OK"
go version && kind version && kubectl version --client && docker version   # prereqs present

# 1. cluster + compiler
make e2e-reset            # kind cluster 'monolift-e2e'
make build-e2e-compile

# 2. full run, processImage only, KEEP pods up, record the output
MONOLIFT_E2E=1 MONOLIFT_E2E_KEEP=1 \
  go test -tags=e2e -v ./test/e2e/... \
  -run 'TestE2E/activation-listmonk-processimage' -timeout=30m \
  2>&1 | tee /tmp/e2e-processimage.log
#   -> expect it to pass through stage 10: byte-identical oracle + fail-mode checks

# 3. discover the live names and fill the table above
kubectl get ns | grep mlv2-lifted-activation-listmonk-processimage
NS=mlv2-lifted-activation-listmonk-processimage-<runid>
kubectl -n $NS get pods
kubectl -n $NS get deploy

# 4. dry-run the live buildserver commands once (Beats 2 & 4 below) so caches are warm
```

Also pre-warm on the **laptop**: open `evaluation/listmonk/cmd/media.go`, the analysis
markdown, and the AdapterPlan JSON in tabs; keep `/tmp/e2e-processimage.log` (or an
asciinema recording) handy as the e2e backup.

---

## The live sequence

### Beat 1 — The buried function  ·  LAPTOP (editor)
Open `evaluation/listmonk/cmd/media.go`, walk the chain in one file:
- `:26` `func (a *App) UploadMedia(...)` — the web handler
- `:99` `thumbFile, wi, he, err := processImage(file)` — the call site
- `:212` `func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)` — the target

Then show how it's wired to a route (terminal, laptop, no compute):
```bash
rg -n "UploadMedia" evaluation/listmonk
```
> Say: "`main()` → route → `UploadMedia` → `processImage`. Monolift recovers this chain for you."

### Beat 2 — Recover the activation path  ·  BUILDSERVER (or show artifact)
**Reliable (laptop, no compute):** open `docs/research/activation-paths/analyses/listmonk-M-4.md`
(path + cut-candidate table + recommendation) and `traces/listmonk-M-4.json`.

**Live (buildserver):**
```bash
ssh monolift-buildserver
cd /local/repository
go build -o /tmp/activation-path ./cmd/activation-path        # build once

# one command — the subshell scopes the cd, so your prompt stays in /local/repository
( cd evaluation/listmonk && /tmp/activation-path \
    --target=cmd/media.go:212 --packages=./... \
    --reverse-import-scope --format=text --verbose )
```
> The `activation-path` binary analyzes its working directory (no `--dir` flag), so the
> `( cd … && … )` subshell runs it inside the listmonk module without moving your shell.
> Say: "edges from `main` to `processImage`, recovered automatically." (`--reverse-import-scope` keeps it light.)

### Beat 3 — The compiler explains itself  ·  LAPTOP
Open `pkg/codegen/testdata/adapter_processimage_plan.golden.json`. This is the money shot:
host vs. remote signature, the two transforms, and **six discharged proof obligations**.
> Say: "It doesn't just generate glue — it emits a checkable plan saying *why it's allowed to*.
> And note `adapter_error_order`: the one observable divergence, named and accounted for."

**Refusal in action (high-impact)  ·  BUILDSERVER.** Show the green plan is *earned* — uncomment one
pre-placed line and watch the compiler refuse instead of shipping something it can't prove faithful.
Uncommenting makes the helper peek at the upload's metadata, which the `multipart_file_read_all`
pattern forbids (it must be used as a pure byte stream — opened once, no filename/size/header). A
comment doesn't affect SSA, so the green plan above is still produced until you uncomment.

The block (already in the laptop copy; place it on the buildbox once — it survives Step 0's idempotent
re-clone because HEAD stays at the pin):
```go
func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {
	// === MONOLIFT DEMO: uncomment the next line to force a refusal (adapter_use_shape) ===
	// if file.Size < 0 { return nil, 0, 0, nil } // peeks at upload metadata; the pattern requires Open()-only
	src, err := file.Open()
	...
```
```bash
# place it on the buildbox (idempotent — safe to re-run):
cd /local/repository/evaluation/listmonk && python3 - <<'PY'
p="cmd/media.go"; s=open(p).read()
if "MONOLIFT DEMO" not in s:
    a="func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {\n\tsrc, err := file.Open()\n"
    s=s.replace(a, "func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {\n\t// === MONOLIFT DEMO: uncomment the next line to force a refusal (adapter_use_shape) ===\n\t// if file.Size < 0 { return nil, 0, 0, nil } // peeks at upload metadata; the pattern requires Open()-only\n\tsrc, err := file.Open()\n", 1)
    open(p,"w").write(s); print("placed")
else: print("already present")
PY
```
Live steps:
1. **Uncomment** — delete the `// ` before `if file.Size` in your editor (or
   `sed -i 's@^\t// if file.Size@\tif file.Size@' cmd/media.go`).
2. **Re-run the lift** — it now refuses:
   ```bash
   cd /local/repository && go run ./cmd lift --source=evaluation/listmonk \
     --target=cmd/media.go:212 --trace=docs/research/activation-paths/traces/listmonk-M-4.json \
     --output=evaluation/listmonk/_monolift_out --service-name=processimage
   ```
   → `Error: missing_reconstructor: reconstructed parameter has no registered reconstructor (*multipart.FileHeader)`
   (exit 1) — vs. the **commented** run, which succeeds (exit 0, emits the plan). That green→red flip is
   the demo. The headline is `missing_reconstructor` **by design**: when the `use_shape` proof fails the
   adapter produces no plan and removes itself, so the cut falls back to the plain, un-reconstructable
   form. The CLI does NOT print the `adapter_use_shape` reason — it lives in `candidate.AdapterReason` /
   `LiftResult.DemotionChain`. Say: *"the adapter backed off — it won't lift what it can't prove faithful."*
   > To put the obligation name on screen, add a one-line `slog.Debug("adapter recovery refused", …,
   > "adapter_reason", candidate.AdapterReason)` in `cut_admit.go` after the `adapterRefusalClass` call
   > (+ `"log/slog"` import), rebuild, and run with `MONOLIFT_LOG_LEVEL=debug` → prints
   > `adapter_reason="adapter_use_shape: helper accesses field on *multipart.FileHeader; only Open() is permitted"`.
3. **Reset** — re-comment the line, or wipe the block entirely with
   `git -C evaluation/listmonk checkout -- cmd/media.go` (note: `git checkout` removes the whole demo
   block — re-run the placement above to restore it).

Note: *every* in-body break surfaces as `missing_reconstructor` for this reason (the adapter rescue is
what's being disabled, not a new headline produced). A second `file.Open()` also trips `use_shape`;
changing the *parameter type* to `io.Reader`/`*os.File`/`chan` would give a different direct refusal but
breaks the `UploadMedia` call site (line 99).
> Say: "The six green checks aren't decoration — uncomment one line and it refuses. It would rather not
> lift than lift something it can't prove behaves identically. That's the soundness argument, live."

### Beat 4 — Watch it synthesize  ·  BUILDSERVER + LAPTOP
```bash
# buildserver, /local/repository
go run ./cmd lift \
  --source=evaluation/listmonk \
  --target=cmd/media.go:212 \
  --trace=docs/research/activation-paths/traces/listmonk-M-4.json \
  --output=evaluation/listmonk/_monolift_out \
  --service-name=processimage \
  --write-monolith-stub
ls -R evaluation/listmonk/_monolift_out  # host stub, normalized server, DTOs, Dockerfiles, manifests
git -C evaluation/listmonk diff -- cmd/media.go   # the ONLY source change: the call site
```
> Say: "Generated both sides + the safety net. The only edit to listmonk is the call site —
> everything else is untouched."

`--output` must be inside the source module (generated code compiles against it; the `_`-prefix keeps
Go tooling from loading it on later runs). Reset the corpus after (so it's clean next time):
```bash
git -C /local/repository/evaluation/listmonk checkout -- cmd/media.go   # undo the call-site patch
rm -rf /local/repository/evaluation/listmonk/_monolift_out              # remove generated output
```

### Beat 5 — It actually runs, served remotely  ·  BUILDSERVER (kubectl) + recording
Pods are already up from pre-warm. The happy path: upload an image, watch the resize happen in the
extracted pod, identical output.
```bash
ssh monolift-buildserver
cd /local/repository
NS=<LIFTED NAMESPACE from table>
kubectl -n $NS get pods            # host (listmonk-lifted) + extracted both Running
```

**Make sure it's routing to the extracted pod** (a full stage-10 run may have toggled the lift off):
```bash
kubectl -n $NS set env deploy/<HOST DEPLOY> \
  MONOLIFT_LIFT_PROCESSIMAGE=on \
  MONOLIFT_LIFT_PROCESSIMAGE_ENDPOINT=http://monolift-extracted-processimage:8081/invoke
kubectl -n $NS rollout status deploy/<HOST DEPLOY>
```

**Upload an image.** Forward the patched monolith's port, then POST a multipart upload to
`/api/media`. That call runs `UploadMedia` → `processImage`, and the resize executes in the
*extracted* pod.
```bash
# session A: forward the host (patched listmonk) to localhost  (background with & or use tmux)
kubectl -n $NS port-forward svc/listmonk-lifted 9000:9000 &

# session B (optional but great): watch the resize land in the extracted pod
kubectl -n $NS logs deploy/<EXTRACTED DEPLOY> -f &

# the upload  (admin/adminpass123; the e2e fixture is a 350x260 PNG)
curl -sS -u admin:adminpass123 \
  -F 'file=@test/e2e/targets/activation_listmonk_processimage/testdata/fixture.png;type=image/png' \
  http://localhost:9000/api/media \
  | jq '.data | {content_type, thumb_url, url, width: .meta.width, height: .meta.height}'
```
> Expect `200`, `width: 350, height: 260`, and a `thumb_url`; the extracted pod's log ticks over on the
> upload. Say: "the resize ran in the *other* process — listmonk didn't notice." Optionally show the
> recording / `/tmp/e2e-processimage.log` where the oracle confirms the lifted thumbnail is byte-identical.

Cleanup the backgrounded forwards when done: `kill %1 %2` (or close the tmux panes).

---

## If something breaks (backups)
- **Beat 2 live fails** → show the checked-in `listmonk-M-4.md` / trace JSON.
- **Beat 4 live fails** → show the AdapterPlan golden (Beat 3) + the committed
  `pkg/codegen/testdata/dto_m4_processimage_*.golden` as "this is what comes out."
- **Beat 5 cluster misbehaves** → the `/tmp/e2e-processimage.log` / recording is the proof.
- **Whole buildserver unreachable** → the entire conceptual arc (Beats 1, 3, + artifacts) runs
  on the laptop with zero compute. You lose only the "live run" flourish.

## Stage ladder (reference, if asked)
0 cluster/namespaces/baseline-image · 1 baseline-deploy · 2 baseline-workload · 3 compile ·
4 verdict · 5 build-lifted-images · 6 load-images · 7 lifted-deploy · 8 lifted-workload ·
9 transcript-compare + env-off-fail-modes · 10 complete.
Stop early with `MONOLIFT_E2E_STOP_STAGE=N` (e.g. `=4` for codegen+verdict, no cluster).

## Teardown (after the talk)
```bash
ssh monolift-buildserver
kind delete cluster --name monolift-e2e
cd /local/repository && make e2e-clean
git -C evaluation/listmonk checkout -- cmd/media.go   # if not already reset
```
