package archtest

import (
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestWireTypesRequireReviewedExternalContractAliases(t *testing.T) {
	const module = "example.com/host"
	const protocolPath = "example.com/host/protocol/relay"
	protocol := types.NewPackage(protocolPath, "relay")
	approved := types.NewNamed(types.NewTypeName(token.NoPos, protocol, "ICEEndpoint", nil), types.NewStruct(nil, nil), nil)
	unaliased := types.NewNamed(types.NewTypeName(token.NoPos, protocol, "DeviceProof", nil), types.NewStruct(nil, nil), nil)
	other := types.NewPackage("example.com/unrelated", "unrelated")
	foreign := types.NewNamed(types.NewTypeName(token.NoPos, other, "Payload", nil), types.NewStruct(nil, nil), nil)
	contract := types.NewPackage(module+"/internal/contract", "contract")
	alias := types.NewAlias(types.NewTypeName(token.NoPos, contract, "RelayICEEndpoint", nil), approved)
	contract.Scope().Insert(alias.Obj())
	foreignAlias := types.NewAlias(types.NewTypeName(token.NoPos, contract, "Foreign", nil), foreign)
	contract.Scope().Insert(foreignAlias.Obj())
	p := &program{
		m:    manifest{module: module, contract: "internal/contract", contractAliases: map[string]string{protocolPath: "published independent protocol module"}},
		pkgs: map[string]*packages.Package{"internal/contract": {Types: contract}},
	}
	for _, tc := range []struct {
		name string
		typ  types.Type
		want bool
	}{
		{"explicit alias", alias, true},
		{"underlying remote identity", approved, true},
		{"collection", types.NewSlice(types.NewPointer(alias)), true},
		{"unaliased sibling", unaliased, false},
		{"unregistered foreign alias", foreignAlias, false},
		{"anonymous shape", types.NewStruct(nil, nil), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if ok, deferred := p.wireType(tc.typ); ok != tc.want || deferred {
				t.Fatalf("wire type accepted=%v deferred=%v, want accepted=%v", ok, deferred, tc.want)
			}
		})
	}
	delete(p.m.contractAliases, protocolPath)
	if ok, _ := p.wireType(alias); ok {
		t.Fatal("an alias bypassed missing module approval")
	}
	p.m.contractAliases[protocolPath] = " \t"
	if ok, _ := p.wireType(alias); ok {
		t.Fatal("a blank reason authorized external wire types")
	}
	p.m.contractAliases[protocolPath] = "published independent protocol module"
	delete(p.pkgs, "internal/contract")
	if ok, _ := p.wireType(approved); ok {
		t.Fatal("module approval alone authorized external wire types")
	}
}

func TestWireTypesDoNotExportPrivateContractAliases(t *testing.T) {
	const module = "example.com/host"
	const remotePath = "example.com/protocol"
	remote := types.NewPackage(remotePath, "protocol")
	payload := types.NewNamed(types.NewTypeName(token.NoPos, remote, "Payload", nil), types.NewStruct(nil, nil), nil)
	contract := types.NewPackage(module+"/internal/contract", "contract")
	alias := types.NewAlias(types.NewTypeName(token.NoPos, contract, "privatePayload", nil), payload)
	contract.Scope().Insert(alias.Obj())
	p := &program{
		m:    manifest{module: module, contract: "internal/contract", contractAliases: map[string]string{remotePath: "reviewed protocol"}},
		pkgs: map[string]*packages.Package{"internal/contract": {Types: contract}},
	}
	if ok, _ := p.wireType(payload); ok {
		t.Fatal("a private alias exported an external wire type")
	}
}
