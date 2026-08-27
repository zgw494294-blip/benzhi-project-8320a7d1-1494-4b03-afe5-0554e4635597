package main

import (
	"bytes"
	"corepreservation/internal/analysis"
	"corepreservation/internal/application"
	"corepreservation/internal/domain"
	"corepreservation/internal/httpapi"
	"corepreservation/internal/store"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19087", "监听地址")
	self := flag.Bool("selfcheck", false, "运行自检")
	flag.Parse()
	if p := os.Getenv("PORT"); p != "" && flag.Lookup("addr").Value.String() == "127.0.0.1:19087" {
		*addr = "127.0.0.1:" + p
	}
	if !safeAddress(*addr) {
		panic("监听地址必须是安全的回环地址，且不得使用常见低位端口")
	}
	dataDir := ".data"
	if *self {
		dataDir = ""
	}
	st, e := store.New(dataDir)
	if e != nil {
		panic(e)
	}
	app := application.New(st)
	_ = analysis.TotalLength(nil)
	srv := &http.Server{Addr: *addr, Handler: httpapi.New(app).Handler(), ReadHeaderTimeout: 2 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second}
	if *self {
		go srv.ListenAndServe()
		time.Sleep(120 * time.Millisecond)
		if e := selfcheck(*addr); e != nil {
			fmt.Println("自检失败:", e)
			os.Exit(1)
		}
		_ = srv.Close()
		fmt.Println("自检通过")
		return
	}
	fmt.Println("服务监听", *addr)
	if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
		panic(e)
	}
}
func req(addr, path string, v any, out any) error {
	b, _ := json.Marshal(v)
	r, e := http.Post("http://"+addr+path, "application/json", bytes.NewReader(b))
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("%s %s: %s", path, r.Status, string(b))
	}
	if out != nil {
		return json.NewDecoder(r.Body).Decode(out)
	}
	return nil
}

func getReq(addr, path string, out any) error {
	r, e := http.Get("http://" + addr + path)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode >= 300 {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("%s %s: %s", path, r.Status, string(b))
	}
	return json.NewDecoder(r.Body).Decode(out)
}
func selfcheck(addr string) error {
	core := domain.CoreRecord{CoreID: fmt.Sprintf("self-core-%d", time.Now().UnixNano()), CatalogCode: "SC", BoxID: "BOX", DepthStartMm: 0, DepthEndMm: 100, InitialMassMg: 1000, AvailableMassMg: 1000, MinimumReserveMassMg: 400, ProtectedIntervals: []domain.Interval{{Start: 10, End: 20}}}
	if e := req(addr, "/api/v1/cores", core, &core); e != nil {
		return e
	}
	c := domain.SamplingCase{CoreID: core.CoreID, Purpose: "同位素分析", Method: "低速切割", RequestedSegments: []domain.Segment{{Start: 30, End: 40}}, EstimatedMassMg: 100}
	var created domain.SamplingCase
	if e := req(addr, "/api/v1/cases", c, &created); e != nil {
		return e
	}
	var pc analysis.PrecheckResult
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/precheck", map[string]any{}, &pc); e != nil {
		return e
	}
	if !pc.Pass {
		return fmt.Errorf("预检未通过")
	}
	var returned domain.SamplingCase
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/review", map[string]string{"code": "DOC", "message": "补充见证人"}, &returned); e != nil {
		return e
	}
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/revise", map[string]any{"expectedRevision": returned.Revision, "purpose": c.Purpose, "method": c.Method, "requestedSegments": c.RequestedSegments, "estimatedMassMg": 100, "revisionNote": "补充见证安排"}, &struct {
		Case *domain.SamplingCase `json:"case"`
	}{Case: &created}); e != nil {
		return e
	}
	for _, f := range created.Findings {
		if e := req(addr, "/api/v1/cases/"+created.CaseID+"/findings/"+f.FindingID+"/close", map[string]any{"closureNote": "已补充", "caseRevision": created.Revision}, nil); e != nil {
			return e
		}
	}
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/precheck", map[string]any{}, &pc); e != nil {
		return e
	}
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/submit", map[string]any{}, &created); e != nil {
		return e
	}
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/authorize", map[string]any{}, &created); e != nil {
		return e
	}
	var ex domain.ExecutionReceipt
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/execute", map[string]any{"idempotencyKey": fmt.Sprintf("self-key-%d", time.Now().UnixNano()), "authorizationDigest": created.AuthorizationDigest, "actualSegments": c.RequestedSegments, "massBeforeMg": 1000, "sampleMassMg": 100, "massAfterMg": 900, "containerCode": "C1", "operator": "实验员", "witness": "复核员"}, &ex); e != nil {
		return e
	}
	var attempt domain.VerificationAttempt
	if e := req(addr, "/api/v1/cases/"+created.CaseID+"/freeze", map[string]any{"remainingMassMg": 900, "storageLocation": "A-1", "verifier": "保管员", "witnessNote": "现场见证", "verificationKey": fmt.Sprintf("verify-%d", time.Now().UnixNano())}, &attempt); e != nil {
		return e
	}
	var vr map[string]any
	return getReq(addr, "/api/v1/credentials/"+attempt.CredentialID, &vr)
}

var _ = strings.TrimSpace
