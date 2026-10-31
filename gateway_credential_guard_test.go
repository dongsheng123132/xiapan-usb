package main

import (
 "context"
 "os"
 "path/filepath"
 "testing"
)

func TestLimitedUIRejectsCorruptWallet(t *testing.T) {
 root := t.TempDir()
 if err := os.MkdirAll(filepath.Join(root,"data","wallet"),0700); err != nil { t.Fatal(err) }
 if err := os.WriteFile(filepath.Join(root,"data","wallet","device.json"),[]byte("{broken"),0600); err != nil { t.Fatal(err) }
 if _,err := resolvePicoCredential(root,true); err == nil { t.Fatal("corrupt wallet treated as missing credential") }
}

func TestStrictGatewayChecksCredentialsBeforeReusingRuntime(t *testing.T) {
 root := t.TempDir()
 // Even after a UI runtime is registered, strict start cannot use it without credentials.
 picoRuntime(root)
 if _,err := ensurePicoGateway(context.Background(),root,false); err == nil { t.Fatal("strict start accepted missing credentials") }
 if _,err := os.Stat(filepath.Join(root,"data")); !os.IsNotExist(err) { t.Fatal("credential rejection created data") }
}
