package main

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestUILanguagePersistenceAndRevision(t *testing.T) {
 root:=t.TempDir()
 initial,err:=loadUISettings(root); if err!=nil {t.Fatal(err)}
 if initial.Language!="auto" {t.Fatal(initial)}
 if _,err=os.Stat(filepath.Join(root,"data"));!os.IsNotExist(err){t.Fatal("reading UI settings created data")}
 saved,err:=uiSettingsAction(root,"settings.ui.save",map[string]any{"language":"en","expected_revision":initial.Revision});if err!=nil{t.Fatal(err)}
 en:=saved.(UISettings)
 if en.Language!="en"||en.Revision==initial.Revision{t.Fatal(en)}
 reloaded,err:=loadUISettings(root);if err!=nil||reloaded!=en{t.Fatalf("reload %v %v",reloaded,err)}
 if _,err=uiSettingsAction(root,"settings.ui.save",map[string]any{"language":"zh-CN","expected_revision":initial.Revision});err==nil{t.Fatal("stale language overwrite accepted")}
 if _,err=uiSettingsAction(root,"settings.ui.save",map[string]any{"language":"fr","expected_revision":en.Revision});err==nil{t.Fatal("unsupported language accepted")}
 if requestLanguage(root,nil)!="en"||requestLanguage(root,map[string]any{"locale":"zh-CN"})!="zh-CN"{t.Fatal("language precedence")}
 if !strings.Contains(responseLanguage("en"),"English"){t.Fatal("response language")}
}

func TestUICorruptSettingsPreserved(t *testing.T){
 root:=t.TempDir(); dir:=filepath.Join(root,"data","settings");if err:=os.MkdirAll(dir,0700);err!=nil{t.Fatal(err)}
 path:=filepath.Join(dir,"ui.json");original:=[]byte("{broken");if err:=os.WriteFile(path,original,0600);err!=nil{t.Fatal(err)}
 if _,err:=uiSettingsAction(root,"settings.ui.save",map[string]any{"language":"en","expected_revision":"initial"});err==nil{t.Fatal("corrupt settings replaced")}
 got,err:=os.ReadFile(path);if err!=nil||string(got)!=string(original){t.Fatal("original settings lost")}
}
