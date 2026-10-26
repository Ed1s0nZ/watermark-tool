package distribution

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"watermark-tool/internal/blind"
)

var testMaster = bytes.Repeat([]byte{42}, 32)
var testLegacy = bytes.Repeat([]byte{7}, 32)
var recipients = []Recipient{{"001", "Alice"}, {"002", "同名"}, {"003", "同名"}}

func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	s, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}
func loadFixture(t *testing.T) image.Image {
	t.Helper()
	img, err := blind.Load("../blind/testdata/clean.png")
	if err != nil {
		t.Fatal(err)
	}
	return img
}
func TestDistributionLifecycle(t *testing.T) {
	s, dir := openTest(t)
	img := loadFixture(t)
	before, _ := os.ReadFile("../blind/testdata/clean.png")
	batch, key, err := s.Begin("request-0000000001", Digest(before), "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "copies.zip")
	if err = Archive(output, img, batch, key); err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(batch.ID, ""); err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	count := 0
	for _, file := range z.File {
		if file.Name == "manifest.csv" {
			continue
		}
		r, _ := file.Open()
		data, _ := io.ReadAll(r)
		r.Close()
		decoded, _, e := image.Decode(bytes.NewReader(data))
		if e != nil {
			t.Fatal(e)
		}
		result, keyID, e := s.Verify(decoded, "")
		if e != nil {
			t.Fatal(e)
		}
		record, e := s.Lookup(result, keyID)
		if e != nil || record == nil || record.ID+".png" != file.Name || record.BatchID != batch.ID {
			t.Fatal(record, e)
		}
		count++
	}
	if count != 3 {
		t.Fatal(count)
	}
	after, _ := os.ReadFile("../blind/testdata/clean.png")
	if !bytes.Equal(before, after) {
		t.Fatal("original mutated")
	}
	retried, _, err := s.Begin("request-0000000001", Digest(before), "admin", recipients)
	if err != nil || retried.Copies[0].ID != batch.Copies[0].ID {
		t.Fatal(retried, err)
	}
	s.Finish(batch.ID, "")
	if _, _, err = s.Begin("request-0000000001", Digest([]byte("different")), "admin", recipients); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	list, total, err := s.Records("002", 1, 1)
	if err != nil || total != 1 || len(list) != 1 || list[0].Name != "同名" {
		t.Fatal(list, total, err)
	}
	list[0].Name = "changed"
	bs, _, _ := s.Batches("", 1, 20)
	bs[0].Copies[0].Name = "changed"
	bs, _, _ = s.Batches("", 1, 20)
	if bs[0].Copies[0].Name != "Alice" {
		t.Fatal("state exposed")
	}
	s.Close()
	reopened, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	bs, total, err = reopened.Batches(batch.ID, 1, 20)
	if err != nil || total != 1 || bs[0].Status != "completed" {
		t.Fatal(bs, total, err)
	}
	if _, _, err = reopened.Resume(batch.ID, "wrong"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestKeysAndMigration(t *testing.T) {
	s, _ := openTest(t)
	golden, err := blind.Load("../blind/testdata/v1.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, id, e := s.Verify(golden, ""); e != nil || id != "legacy-env" {
		t.Fatal(id, e)
	}
	k, err := s.CreateKey("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetKeyState(k.ID, "active"); err != nil {
		t.Fatal(err)
	}
	if _, id, e := s.Verify(golden, ""); e != nil || id != "legacy-env" {
		t.Fatal("old version lost after rotation", id, e)
	}
	if _, err = s.SetKeyState(k.ID, "disabled"); !errors.Is(err, ErrState) {
		t.Fatal(err)
	}
	if _, err = s.SetKeyState("legacy-env", "disabled"); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Verify(golden, "legacy-env"); !errors.Is(err, ErrDisabled) {
		t.Fatal(err)
	}
	if _, _, err = s.Verify(golden, ""); err == nil {
		t.Fatal("disabled key used")
	}
	keys, _ := s.Keys()
	encoded, _ := json.Marshal(keys)
	if bytes.Contains(encoded, []byte("secret")) {
		t.Fatal("secret exposed")
	}
	// Correctly signed ordinary text is not automatically a registered distribution.
	_, secret, _ := s.Current()
	marked, _ := blind.Embed(loadFixture(t), "ordinary text", secret, time.Now())
	result, id, e := s.Verify(marked, "")
	if e != nil {
		t.Fatal(e)
	}
	record, e := s.Lookup(result, id)
	if e != nil || record != nil {
		t.Fatal(record, e)
	}
}
func TestCrashCorruptionLockAndWriteFailure(t *testing.T) {
	s, dir := openTest(t)
	if second, err := Open(dir, testMaster, testLegacy); err == nil {
		second.Close()
		t.Fatal("second writer accepted")
	}
	batch, _, err := s.Begin("request-0000000002", strings.Repeat("a", 64), "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	recovered, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	rows, _, _ := recovered.Batches("", 1, 20)
	if rows[0].Status != "failed" || rows[0].Error != "interrupted" {
		t.Fatal(rows)
	}
	next, _, err := recovered.Resume(batch.ID, strings.Repeat("a", 64))
	if err != nil || next.ID != batch.ID {
		t.Fatal(next, err)
	}
	recovered.Finish(batch.ID, "")
	oldData, _ := os.ReadFile(filepath.Join(dir, "platform.json"))
	recovered.write = func(string, []byte) error { return os.ErrPermission }
	if _, _, err = recovered.Begin("request-0000000003", strings.Repeat("a", 64), "admin", recipients); !errors.Is(err, ErrStore) {
		t.Fatal(err)
	}
	if _, _, err = recovered.Records("", 1, 20); !errors.Is(err, ErrStore) {
		t.Fatal(err)
	}
	recovered.Close()
	data, _ := os.ReadFile(filepath.Join(dir, "platform.json"))
	if !bytes.Equal(data, oldData) {
		t.Fatal("failed write changed durable state")
	}
	if wrong, err := Open(dir, bytes.Repeat([]byte{43}, 32), testLegacy); err == nil {
		wrong.Close()
		t.Fatal("wrong master accepted")
	}
	data, _ = os.ReadFile(filepath.Join(dir, "platform.json"))
	if !bytes.Equal(data, oldData) {
		t.Fatal("wrong master mutated records")
	}
	os.WriteFile(filepath.Join(dir, "platform.json"), []byte("broken"), 0600)
	if wrong, err := Open(dir, testMaster, testLegacy); err == nil {
		wrong.Close()
		t.Fatal("corrupt state silently reset")
	}
}
func TestConcurrentIdempotencyAndCSV(t *testing.T) {
	s, _ := openTest(t)
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := s.Begin("request-0000000004", strings.Repeat("b", 64), "admin", recipients)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				accepted++
			} else if !errors.Is(err, ErrBusy) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if accepted != 1 {
		t.Fatal(accepted)
	}
	for _, csv := range []string{"", "recipient_id,name\n1,A\n1,B", "wrong,name\n1,A", "recipient_id,name\n,A"} {
		if _, err := ParseRecipients(csv); !errors.Is(err, ErrRecipients) {
			t.Fatal(csv, err)
		}
	}
	parsed, err := ParseRecipients("\ufeffrecipient_id,name\n1,\"Alice, Bob\"\n2,名字")
	if err != nil || len(parsed) != 2 {
		t.Fatal(parsed, err)
	}
	writer := &limitedWriter{writer: io.Discard, remaining: 2}
	if _, err = writer.Write([]byte("long")); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if csvCell("=SUM(1,2)") != "'=SUM(1,2)" {
		t.Fatal("formula not escaped")
	}
}

func TestPinnedKeysAndRegistrationBinding(t *testing.T) {
	s, _ := openTest(t)
	img := loadFixture(t)
	batch, key, err := s.Begin("request-0000000005", strings.Repeat("c", 64), "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.CreateKey("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SetKeyState(next.ID, "active"); err != nil {
		t.Fatal(err)
	}
	marked, err := blind.Embed(img, batch.Copies[0].ID, key, batch.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(batch.ID, ""); err != nil {
		t.Fatal(err)
	}
	result, id, err := s.Verify(marked, "")
	if err != nil || id != "legacy-env" {
		t.Fatal(id, err)
	}
	if record, err := s.Lookup(result, id); err != nil || record == nil {
		t.Fatal(record, err)
	}
	changed := result
	changed.Timestamp = batch.CreatedAt.Add(time.Second).Format(time.RFC3339)
	if record, err := s.Lookup(changed, id); err != nil || record != nil {
		t.Fatal("timestamp mismatch attributed", record, err)
	}
	if record, err := s.Lookup(result, next.ID); err != nil || record != nil {
		t.Fatal("key mismatch attributed", record, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	output := filepath.Join(t.TempDir(), "cancelled.zip")
	if err := ArchiveContext(ctx, output, img, batch, key); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("partial archive retained")
	}
}
func TestKeyLimitsAndPrivateState(t *testing.T) {
	s, dir := openTest(t)
	for i := 0; i < 7; i++ {
		if _, err := s.CreateKey("admin"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateKey("admin"); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	keys, _ := s.Keys()
	if _, err := s.SetKeyState(keys[1].ID, "disabled"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateKey("admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetKeyState(keys[1].ID, "decrypt-only"); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	encrypted, _ := os.ReadFile(filepath.Join(dir, "keys.enc"))
	if bytes.Contains(encrypted, testLegacy) || bytes.Contains(encrypted, []byte("legacy-env")) {
		t.Fatal("key material not encrypted")
	}
	s.Close()
	wrong, err := Open(filepath.Join(t.TempDir(), "same"), testLegacy, testLegacy)
	if err == nil {
		wrong.Close()
		t.Fatal("master key reuse accepted")
	}
}

func TestRegenerationFailurePreservesRegistration(t *testing.T) {
	s, _ := openTest(t)
	b, key, err := s.Begin("request-0000000006", strings.Repeat("d", 64), "admin", recipients)
	if err != nil {
		t.Fatal(err)
	}
	img, err := blind.Embed(loadFixture(t), b.Copies[0].ID, key, b.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Finish(b.ID, ""); err != nil {
		t.Fatal(err)
	}
	result, id, err := s.Verify(img, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Resume(b.ID, b.SourceSHA256); err != nil {
		t.Fatal(err)
	}
	record, err := s.Lookup(result, id)
	if err != nil || record == nil {
		t.Fatal("regeneration hid existing registration", record, err)
	}
	if err = s.Finish(b.ID, "generation_failed"); err != nil {
		t.Fatal(err)
	}
	record, err = s.Lookup(result, id)
	if err != nil || record == nil {
		t.Fatal("failed regeneration revoked registration", record, err)
	}
}

func TestRuntimeCleanupIsSeparateFromDurableState(t *testing.T) {
	s, dir := openTest(t)
	runtime := s.RuntimeDir()

	orphan := filepath.Join(dir, ".txn-interrupted")
	if err := os.WriteFile(orphan, []byte("partial metadata transaction"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtime, "orphan-image"), []byte("private temporary input"), 0600); err != nil {
		t.Fatal(err)
	}
	keys, _ := os.ReadFile(filepath.Join(dir, "keys.enc"))
	s.Close()
	wrong, err := Open(dir, bytes.Repeat([]byte{43}, 32), testLegacy)
	if err == nil {
		wrong.Close()
		t.Fatal("wrong master accepted")
	}
	if _, err := os.Stat(filepath.Join(runtime, "orphan-image")); err != nil {
		t.Fatal("bad-key startup modified runtime")
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatal("bad-key startup removed transaction", err)
	}
	reopened, err := Open(dir, testMaster, testLegacy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	entries, err := os.ReadDir(runtime)
	if err != nil || len(entries) != 0 {
		t.Fatal("orphan temp not cleaned", err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatal("orphan transaction not cleaned", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "keys.enc"))
	if !bytes.Equal(keys, after) {
		t.Fatal("runtime cleanup modified keys")
	}
}
