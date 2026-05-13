package codegen

import (
	"go/types"
	"testing"

	"github.com/tgoodwin/monolift/pkg/activation"
)

// makeTestPkg creates a throw-away package for synthetic types.
func makeTestPkg(name string) *types.Package {
	return types.NewPackage("example.com/test/"+name, name)
}

// makeNamedStruct creates a named struct type in the given package.
func makeNamedStruct(pkg *types.Package, name string, fields ...*types.Var) *types.Named {
	strct := types.NewStruct(fields, nil)
	tn := types.NewTypeName(0, pkg, name, nil)
	named := types.NewNamed(tn, strct, nil)
	pkg.Scope().Insert(tn)
	return named
}

func TestSelectReceiverPolicySerializableBoundary(t *testing.T) {
	pkg := makeTestPkg("configpkg")
	named := makeNamedStruct(pkg, "SerializableConfig",
		types.NewField(0, pkg, "Name", types.Typ[types.String], false),
		types.NewField(0, pkg, "Count", types.Typ[types.Int], false),
		types.NewField(0, pkg, "Enabled", types.Typ[types.Bool], false),
	)
	spec, err := selectReceiverPolicy(named, false, activation.Stateless)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Policy != ReceiverBoundary {
		t.Fatalf("policy = %s, want %s", spec.Policy, ReceiverBoundary)
	}
	if spec.IsPointer {
		t.Fatal("IsPointer = true, want false")
	}
	if spec.GoType != "SerializableConfig" {
		t.Fatalf("GoType = %s, want SerializableConfig", spec.GoType)
	}
	if spec.Codec != CodecJSON {
		t.Fatalf("Codec = %s, want %s", spec.Codec, CodecJSON)
	}
}

func TestSelectReceiverPolicyFactory(t *testing.T) {
	// Register a test factory entry.
	key := "example.com/test/factorypkg.FactoryBuilt"
	receiverFactoryRegistry[key] = receiverFactoryEntry{FactoryFunc: "NewFactoryBuilt"}
	defer delete(receiverFactoryRegistry, key)

	pkg := makeTestPkg("factorypkg")
	named := makeNamedStruct(pkg, "FactoryBuilt",
		types.NewField(0, pkg, "secret", types.Typ[types.String], false), // unexported
		types.NewField(0, pkg, "count", types.Typ[types.Int], false),     // unexported
	)
	spec, err := selectReceiverPolicy(named, true, activation.Stateless)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Policy != ReceiverFactory {
		t.Fatalf("policy = %s, want %s", spec.Policy, ReceiverFactory)
	}
	if !spec.IsPointer {
		t.Fatal("IsPointer = false, want true")
	}
	if spec.FactoryFunc != "NewFactoryBuilt" {
		t.Fatalf("FactoryFunc = %s, want NewFactoryBuilt", spec.FactoryFunc)
	}
}

func TestSelectReceiverPolicyRefusedDBField(t *testing.T) {
	pkg := makeTestPkg("dbpkg")
	sqlPkg := types.NewPackage("database/sql", "sql")
	dbType := types.NewTypeName(0, sqlPkg, "DB", nil)
	dbNamed := types.NewNamed(dbType, types.NewStruct(nil, nil), nil)

	named := makeNamedStruct(pkg, "HasDBField",
		types.NewField(0, sqlPkg, "DB", types.NewPointer(dbNamed), false),
		types.NewField(0, pkg, "Name", types.Typ[types.String], false),
	)
	_, err := selectReceiverPolicy(named, true, activation.Stateless)
	if err == nil {
		t.Fatal("expected refusal, got nil error")
	}
	if want := "receiver_requires_reconstruction"; !stringContains(err.Error(), want) {
		t.Fatalf("error = %q, want to contain %q", err.Error(), want)
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
