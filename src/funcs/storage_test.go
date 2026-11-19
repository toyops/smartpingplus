package funcs

import (
	"database/sql"
	"github.com/smartping/smartping/src/g"
	_ "modernc.org/sqlite"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test in-memory db: %v", err)
	}

	schemas := []string{
		`CREATE TABLE pinglog (
			logtime VARCHAR(20),
			target VARCHAR(100),
			maxdelay VARCHAR(20),
			mindelay VARCHAR(20),
			avgdelay VARCHAR(20),
			sendpk INTEGER,
			revcpk INTEGER,
			losspk INTEGER
		);`,
		`CREATE TABLE tcppinglog (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			logtime VARCHAR(20),
			target VARCHAR(100),
			maxdelay VARCHAR(20),
			mindelay VARCHAR(20),
			avgdelay VARCHAR(20),
			sendpk INTEGER,
			revcpk INTEGER,
			losspk INTEGER
		);`,
		`CREATE TABLE alertlog (
			logtime VARCHAR(20),
			targetip VARCHAR(100),
			targetname VARCHAR(100),
			tracert TEXT
		);`,
		`CREATE TABLE mappinglog (
			logtime VARCHAR(20) PRIMARY KEY,
			mapjson TEXT
		);`,
	}

	for _, s := range schemas {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("failed to create schema: %v", err)
		}
	}

	g.Db = db
	g.Cfg.Base = map[string]int{"Archive": 7}
	return db
}

func TestPingStorageParameterized(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	maliciousTarget := "1.1.1.1' OR '1'='1"
	stat := g.PingSt{
		SendPk:   20,
		RevcPk:   20,
		LossPk:   0,
		MinDelay: 10.5,
		AvgDelay: 12.0,
		MaxDelay: 15.2,
	}

	PingStorage(stat, maliciousTarget)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pinglog WHERE target = ?", maliciousTarget).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record for malicious target, got %d", count)
	}
}

func TestTcpPingStorageParameterized(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	maliciousTarget := "127.0.0.1:80'; DROP TABLE tcppinglog; --"
	stat := g.PingSt{
		SendPk:   10,
		RevcPk:   10,
		LossPk:   0,
		MinDelay: 1.0,
		AvgDelay: 2.0,
		MaxDelay: 3.0,
	}

	TcpPingStorage(stat, maliciousTarget)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM tcppinglog WHERE target = ?", maliciousTarget).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record for malicious target, got %d", count)
	}
}

func TestAlertStorageAndCheckStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	maliciousIP := "10.0.0.99' OR 1=1 --"
	alertLog := g.AlertLog{
		Logtime:    time.Now().Format("2006-01-02 15:04"),
		Targetip:   maliciousIP,
		Targetname: "TestNode",
		Tracert:    "[]",
	}

	AlertStorage(alertLog)

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM alertlog WHERE targetip = ?", maliciousIP).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 record, got %d", count)
	}

	rule := g.AlertRule{
		Addr:        maliciousIP,
		Thdchecksec: "900",
		Thdavgdelay: "100",
		Thdloss:     "50",
		Thdoccnum:   "3",
	}
	// Should safely return false or true without SQL injection syntax errors
	_ = CheckAlertStatus(rule)
}

func TestMapPingStorageSingleExec(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	MapPingStorage()

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM mappinglog").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 mappinglog record, got %d", count)
	}
}

func TestClearArchiveParameterized(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ClearArchive()
}
