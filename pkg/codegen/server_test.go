package codegen

import (
	"bytes"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRenderServerRefreshFeedGolden(t *testing.T) {
	fixture := RefreshFeedFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "miniflux_refreshfeed_server.go.golden")
	if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rendered server does not match %s", goldenPath)
	}
	assertRenderedContains(t, got,
		`cfg := config.NewConfigParser()`,
		`config.Opts = opts`,
		`storeDB, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
		`if err := storeDB.PingContext(context.Background()); err != nil {`,
		`state.Store = storage.NewStorage(storeDB)`,
		`state.closeFuncs = append(state.closeFuncs, func() error { return storeDB.Close() })`,
		`defer state.Close()`,
	)
}

func streamingBytesServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-processstream",
		EnvServiceName:   "PROCESSSTREAM",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/stream",
			PackageName: "stream",
			FuncName:    "ProcessStream",
		},
		BoundaryParams: []Param{
			{Name: "baseURL", JSONName: "base_url", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "r", JSONName: "r", GoType: "io.ReadSeeker", QualifiedGoType: "io.ReadSeeker", TypePackagePath: "io", Codec: CodecStreamingBytes, Index: 1},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "string"},
		ServerPath:  "/tmp/test/cmd/monolift-processstream/main.go",
	}
}

func TestRenderServerStreamingBytesGolden(t *testing.T) {
	plan := streamingBytesServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "streaming_bytes_server.go.golden")
	if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rendered server does not match %s\ngot:\n%s", goldenPath, got)
	}
}

func TestRenderServerSQLReconstructorIncludesPQBlankImport(t *testing.T) {
	plan := &Plan{
		ServiceName:      "monolift-query",
		EnvServiceName:   "QUERY",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/query",
			PackageName: "query",
			FuncName:    "Run",
		},
		ReconstructedParams: []ReconstructedParam{{
			Param: Param{
				Name:             "db",
				JSONName:         "db",
				GoType:           "*sql.DB",
				QualifiedGoType:  "*sql.DB",
				TypePackagePath:  "database/sql",
				TypePackageAlias: "sql",
				Codec:            CodecJSON,
				Index:            0,
			},
			Reconstructor: Reconstructor{
				ID:      "sql_db",
				Type:    "*database/sql.DB",
				Imports: []string{"database/sql", "os", "_ github.com/lib/pq"},
			},
		}},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "string"},
		ServerPath:  "/tmp/test/cmd/monolift-query/main.go",
	}
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	if !bytes.Contains(got, []byte("_ \"github.com/lib/pq\"")) {
		t.Fatalf("rendered server missing lib/pq blank import:\n%s", got)
	}
}

func directSQLDBServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-query",
		EnvServiceName:   "QUERY",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/query",
			PackageName: "query",
			FuncName:    "Run",
		},
		ReconstructedParams: []ReconstructedParam{{
			Param: Param{
				Name:             "db",
				JSONName:         "db",
				GoType:           "*sql.DB",
				QualifiedGoType:  "*sql.DB",
				TypePackagePath:  "database/sql",
				TypePackageAlias: "sql",
				Codec:            CodecJSON,
				Index:            0,
			},
			Reconstructor: Reconstructor{
				ID:          "sql_db",
				Type:        "*database/sql.DB",
				Imports:     []string{"context", "database/sql", "os", "_ github.com/lib/pq"},
				CloseSource: "db.Close()",
			},
		}},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "string"},
		ServerPath:  "/tmp/test/cmd/monolift-query/main.go",
	}
}

func filesystemReceiverPlan() *Plan {
	recon := filesystemSystemReconstructor(types.NewPointer(pocketbaseFilesystemSystemType()))
	plan := &Plan{
		ServiceName:      "create-thumb",
		EnvServiceName:   "CREATE_THUMB",
		SourceModuleRoot: "/tmp/source",
		SourceModulePath: "github.com/pocketbase/pocketbase",
		CutPoint: CutPoint{
			PackagePath: "github.com/pocketbase/pocketbase/tools/filesystem",
			PackageName: "filesystem",
			FuncName:    "CreateThumb",
			Receiver:    "*System",
		},
		ReceiverParam: &ReceiverSpec{
			GoType:        "*System",
			IsPointer:     true,
			Policy:        ReceiverReconstructed,
			Reconstructor: recon,
		},
		BoundaryParams: []Param{
			{Name: "originalKey", JSONName: "original_key", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "thumbKey", JSONName: "thumb_key", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 1},
			{Name: "thumbSize", JSONName: "thumb_size", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 2},
		},
		Results: []Result{
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecError, GoType: "error"},
		ServerPath:  "/tmp/source/.monolift-create-thumb/cmd/create-thumb/main.go",
	}
	applyLiftOptions(plan, LiftOptions{Output: "/tmp/source/.monolift-create-thumb", ServiceName: "create-thumb"})
	return plan
}

func TestRenderServerFilesystemReceiverGolden(t *testing.T) {
	plan := filesystemReceiverPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "filesystem_receiver_server.go.golden")
	if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rendered server does not match %s\ngot:\n%s", goldenPath, got)
	}
	assertRenderedContains(t, got,
		`Receiver    *filesystem.System`,
		`receiverRoot := os.Getenv("MONOLIFT_FILESYSTEM_ROOT")`,
		`if err := os.MkdirAll(receiverCleanRoot, 0o755); err != nil {`,
		`receiverRootInfo, err := os.Stat(receiverCleanRoot)`,
		`receiver, err := filesystem.NewLocal(receiverCleanRoot)`,
		`state.Receiver = receiver`,
		`state.closeFuncs = append(state.closeFuncs, func() error { return state.Receiver.Close() })`,
		`monoliftValidateRootRelativePath("original_key", req.OriginalKey)`,
		`monoliftValidateRootRelativePath("thumb_key", req.ThumbKey)`,
		`resultErr := filesystem.MonoliftInvokeCreateThumb(state.Receiver, req.OriginalKey, req.ThumbKey, req.ThumbSize)`,
	)
	if bytes.Contains(got, []byte(`monoliftValidateRootRelativePath("thumb_size"`)) {
		t.Fatalf("thumb_size should not be root-relative validated:\n%s", got)
	}
}

func TestRenderServerDirectSQLDBGolden(t *testing.T) {
	plan := directSQLDBServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "direct_sql_db_server.go.golden")
	if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rendered server does not match %s\ngot:\n%s", goldenPath, got)
	}
	assertRenderedContains(t, got,
		`dbDB, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
		`if err := dbDB.PingContext(context.Background()); err != nil {`,
		`state.Db = dbDB`,
		`state.closeFuncs = append(state.closeFuncs, func() error { return dbDB.Close() })`,
		`defer state.Close()`,
	)
}

func assertRenderedContains(t *testing.T, got []byte, snippets ...string) {
	t.Helper()
	for _, snippet := range snippets {
		if !bytes.Contains(got, []byte(snippet)) {
			t.Fatalf("rendered server missing %q:\n%s", snippet, got)
		}
	}
}

func TestRenderedRefreshFeedServerGoVet(t *testing.T) {
	root := repoRoot(t)
	sourceCopy := copySourceToTemp(t, filepath.Join(root, "evaluation", "miniflux"))
	fixture := RefreshFeedFixtureWithSource(root, sourceCopy)
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(sourceCopy, ".monolift-vet-refreshfeed")
	applyLiftOptions(plan, LiftOptions{Output: output, ServiceName: "refreshfeed"})
	plan.Admission = AdmissionVerdict{Accepted: true, Reasons: []string{"vet test"}}
	serverFiles, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	clientFiles, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	serverArtifacts := artifactsFromRendered("server", serverFiles)
	if _, err := writeArtifactFiles(plan, serverArtifacts); err != nil {
		t.Fatal(err)
	}
	stubContent := clientFiles[plan.ClientPath]
	if _, err := PatchCutFunction(plan, stubContent); err != nil {
		t.Fatal(err)
	}
	// Write adapter after patching so monoliftOriginalRefreshFeed exists.
	adapterFiles, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}
	for adapterPath, content := range adapterFiles {
		if err := os.WriteFile(adapterPath, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "vet", "./cmd/"+plan.ServiceName)
	cmd.Dir = plan.OutputDir
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/monolift-gocache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet generated server: %v\n%s", err, out)
	}
}
