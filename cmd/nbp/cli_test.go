package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// Mock repository for testing CLI
type mockRepo struct {
	rate float64
	err  error
}

func (m *mockRepo) LoadData() error {
	return nil
}

func (m *mockRepo) GetRate(currency string, date time.Time) (float64, error) {
	return m.rate, m.err
}

// Mock downloader
type mockDownloader struct {
	called bool
}

func (m *mockDownloader) UpdateData(start, end int) error {
	m.called = true
	return nil
}

func TestRunCLI_Update(t *testing.T) {
	out := new(bytes.Buffer)
	repo := &mockRepo{}
	updater := &mockDownloader{}

	args := []string{"nbp", "update"}
	err := run(args, out, repo, updater)
	if err != nil {
		t.Fatalf("run(update) failed: %v", err)
	}

	if !updater.called {
		t.Error("Updater was not called")
	}
	if !strings.Contains(out.String(), "XXXX Update complete") {
		t.Errorf("Unexpected output: %s", out.String())
	}
}

func TestRunCLI_Query(t *testing.T) {
	out := new(bytes.Buffer)
	repo := &mockRepo{rate: 4.05, err: nil}
	updater := &mockDownloader{}

	args := []string{"nbp", "USD", "2024-01-01"}
	err := run(args, out, repo, updater)
	if err != nil {
		t.Fatalf("run(query) failed: %v", err)
	}

	if strings.Contains(out.String(), "Rate for USD on 2024-01-01: 4.0500") == false { // Explicit check
		t.Errorf("Output checking failed. Got: %s", out.String())
	}
}

func TestRunCLI_Query_Error(t *testing.T) {
	out := new(bytes.Buffer)
	repo := &mockRepo{rate: 0, err: errors.New("not found")}
	updater := &mockDownloader{}

	args := []string{"nbp", "XYZ"}
	err := run(args, out, repo, updater)
	if err == nil {
		t.Error("run(query) should fail for unknown currency")
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		args     []string
		wantCmd  string
		wantCur  string
		wantDate string
		wantErr  bool
	}{
		{[]string{"nbp", "USD"}, "query", "USD", "", false},
		{[]string{"nbp", "USD", "2024-01-01"}, "query", "USD", "2024-01-01", false},
		{[]string{"nbp", "update"}, "update", "", "", false},
		{[]string{"nbp", "-a", "EUR"}, "query", "EUR", "", false},
		{[]string{"nbp"}, "", "", "", true}, // Missing args
	}

	for _, tt := range tests {
		cmd, cur, dat, err := parseArgs(tt.args)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			continue
		}
		if cmd != tt.wantCmd {
			t.Errorf("parseArgs(%v) cmd = %s, want %s", tt.args, cmd, tt.wantCmd)
		}
		if cur != tt.wantCur {
			t.Errorf("parseArgs(%v) cur = %s, want %s", tt.args, cur, tt.wantCur)
		}
		if dat != tt.wantDate {
			t.Errorf("parseArgs(%v) dat = %s, want %s", tt.args, dat, tt.wantDate)
		}
	}
}
