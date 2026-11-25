package document

import (
	pb "collaboration/contracts/internalv1"
	domain "collaboration/document/internal/domain/document"
	logic "collaboration/document/internal/logic/document"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// 暂停发送方第一次设置期限，实际取消先设立即过期，再让原发送调用继续。
// 包装器只控制调度；期限和写入均交给真实 net/http TLS ResponseWriter。
type deadlineRaceWriter struct {
	http.ResponseWriter
	started       chan struct{}
	continueWrite chan struct{}
	cancelApplied chan struct{}
	calls         atomic.Int32
}

func (w *deadlineRaceWriter) SetWriteDeadline(deadline time.Time) error {
	if w.calls.Add(1) == 1 {
		close(w.started)
		<-w.continueWrite
		return http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
	}
	err := http.NewResponseController(w.ResponseWriter).SetWriteDeadline(deadline)
	close(w.cancelApplied)
	return err
}

func (w *deadlineRaceWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func TestTransportCancellationCannotBeOverwrittenBySendDeadline(t *testing.T) {
	t.Run("JSON", func(t *testing.T) { transportCancellation(t, false) })
	t.Run("Resource", func(t *testing.T) { transportCancellation(t, true) })
}

func transportCancellation(t *testing.T, binary bool) {
	// 用已关闭的真实 database/sql 池隔离持久结算：禁止访问正式私有库。
	// 本例证明实际 TLS 输出边界，不把 Local.Stop 结果当跨服务持久 ACK。
	database, err := sql.Open("mysql", "/unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	self := domain.OutputInstance{ID: "unit-document"}
	manager := logic.NewOutputs(database, nil, self, nil, nil)
	source := &pb.SourceEpoch{ScopeKey: "identity:7", Epoch: 1}
	started := make(chan struct{})
	continueWrite := make(chan struct{})
	cancelApplied := make(chan struct{})
	sendResult := make(chan error, 1)
	const privateText = "取消之后不能发送这段私有正文"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &deadlineRaceWriter{ResponseWriter: w, started: started, continueWrite: continueWrite, cancelApplied: cancelApplied}
		output := NewOutputs(manager).Bind(writer, &logic.ResponseRegistration{Local: domain.OutputLease{ID: "unit-output", Instance: self.ID}}, []*pb.SourceEpoch{source})
		if binary {
			_, err := output.SendResource(r.Context(), &logic.ResourceDownload{ID: 1, Size: uint64(len(privateText)), MediaType: "image/png", Reader: io.NopCloser(strings.NewReader(privateText))})
			sendResult <- err
		} else {
			sendResult <- output.Send(r.Context(), 200, map[string]string{"body": privateText})
		}
	}))
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(continueWrite) }) }
	defer func() {
		release()
		server.Close()
	}()
	client := server.Client()
	client.Timeout = 3 * time.Second
	responseResult := make(chan struct {
		body string
		err  error
	}, 1)
	go func() {
		response, err := client.Get(server.URL)
		if err != nil {
			responseResult <- struct {
				body string
				err  error
			}{err: err}
			return
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		responseResult <- struct {
			body string
			err  error
		}{body: string(raw), err: err}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("TLS sender did not reach deadline boundary")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- manager.Local().Stop(ctx, source) }()
	select {
	case <-cancelApplied:
	case <-ctx.Done():
		t.Fatal("actual cancellation deadline was not applied")
	}
	select {
	case <-stopped:
		t.Fatal("local stop completed before actual sender returned")
	default:
	}
	release()
	result := <-responseResult
	sendErr := <-sendResult
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
	// 稳定的原始正文断言：未关闭但正在发送与已经取消必须区别处理。
	if strings.Contains(result.body, privateText) {
		t.Fatal("actual TLS response released private body after cancellation")
	}
	if sendErr == nil {
		t.Fatal("cancelled output reported successful send")
	}
	if !manager.Local().Has("unit-output") {
		t.Fatal("unavailable durable settlement must remain retained")
	}
}

func closedOutputManager(t *testing.T) *logic.Outputs {
	t.Helper()
	database, err := sql.Open("mysql", "/unused")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	return logic.NewOutputs(database, nil, domain.OutputInstance{ID: "unit-document"}, nil, nil)
}

func TestTransportOpenOutputSendsCompleteTLSBody(t *testing.T) {
	manager := closedOutputManager(t)
	const outputID = "healthy-output"
	body := strings.Repeat("正常中文正文", 32768)
	sent := make(chan error, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		registration := &logic.ResponseRegistration{Local: domain.OutputLease{ID: outputID, Instance: manager.Self().ID}}
		output := NewOutputs(manager).Bind(w, registration, []*pb.SourceEpoch{{ScopeKey: "identity:8", Epoch: 1}})
		sent <- output.Send(r.Context(), 200, map[string]string{"body": body})
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 3 * time.Second
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]string
	if err := json.Unmarshal(raw, &actual); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || actual["body"] != body {
		t.Fatal("healthy actual TLS body was truncated or refused")
	}
	if err := <-sent; err != nil {
		t.Fatal(err)
	}
	if !manager.Local().Has(outputID) {
		t.Fatal("transport completion cannot invent durable settlement success")
	}
}

type pausedOutputBody struct {
	started chan struct{}
	release <-chan struct{}
}

func (body pausedOutputBody) MarshalJSON() ([]byte, error) {
	close(body.started)
	<-body.release
	return json.Marshal(map[string]string{"body": "构造期间被取消的私有正文"})
}

func TestTransportCancellationWhileBuildingBodyDoesNotReleaseIt(t *testing.T) {
	manager := closedOutputManager(t)
	started, releaseBody := make(chan struct{}), make(chan struct{})
	source := &pb.SourceEpoch{ScopeKey: "identity:9", Epoch: 1}
	sent := make(chan error, 1)
	bound := make(chan *ResponseOutput, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		registration := &logic.ResponseRegistration{Local: domain.OutputLease{ID: "building-output", Instance: manager.Self().ID}}
		output := NewOutputs(manager).Bind(w, registration, []*pb.SourceEpoch{source})
		bound <- output
		sent <- output.Send(r.Context(), 200, pausedOutputBody{started: started, release: releaseBody})
	}))
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseBody) }) }
	defer func() {
		release()
		server.Close()
	}()
	client := server.Client()
	client.Timeout = 3 * time.Second
	result := make(chan string, 1)
	go func() {
		response, err := client.Get(server.URL)
		if err != nil {
			result <- ""
			return
		}
		defer response.Body.Close()
		raw, _ := io.ReadAll(response.Body)
		result <- string(raw)
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("actual sender did not begin body construction")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- manager.Local().Stop(ctx, source) }()
	// 观察真实输出绑定的取消信号，不能靠固定毫秒猜时序。
	output := <-bound
	select {
	case <-output.stopped:
	case <-ctx.Done():
		t.Fatal("actual output did not receive cancellation")
	}
	// 未释放Marshal时，实际输出尚未结束，Stop应仍在等待。
	select {
	case <-stopped:
		t.Fatal("stop completed while actual response body was still being constructed")
	default:
	}
	release()
	if strings.Contains(<-result, "构造期间被取消的私有正文") {
		t.Fatal("cancelled constructed body reached the actual TLS client")
	}
	if err := <-sent; err == nil {
		t.Fatal("cancelled constructed body reported send success")
	}
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
}
