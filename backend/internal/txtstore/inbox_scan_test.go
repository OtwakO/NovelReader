package txtstore

import (
	"crypto/rand"
	"os"
	"testing"
)

func TestInboxPagesAreBoundedAndKeepClaimedNamesVisible(t *testing.T) {
	store, _, home, _ := receiptStore(t)
	inbox, err := home.Files().OpenInbox()
	if err != nil {
		t.Fatal(err)
	}
	defer inbox.Close()
	for _, name := range []string{"z.txt", "a.TXT", "middle.txt", "copy.txt.part", "other.epub"} {
		if err := inbox.WriteFile(name, []byte(novel), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := inbox.Mkdir("directory.txt", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := inbox.Symlink("z.txt", "link.txt"); err != nil {
		t.Fatal(err)
	}
	id := rand.Text()
	if _, err := home.DB().Exec(`INSERT INTO txt_inbox_claims(name,receipt_id) VALUES(?,?)`, "middle.txt", id); err != nil {
		t.Fatal(err)
	}
	page, err := store.ScanInbox(t.Context(), "", 2)
	if err != nil || len(page) != 2 || page[0].Name != "a.TXT" || page[1].Name != "directory.txt" || page[1].Problem != "not_regular" {
		t.Fatalf("page: %+v %v", page, err)
	}
	next, err := store.ScanInbox(t.Context(), page[1].Name, 2)
	if err != nil || len(next) != 2 || next[0].Name != "link.txt" || next[0].Problem != "not_regular" || next[1].ReceiptID != id {
		t.Fatalf("next: %+v %v", next, err)
	}
	if _, err := inbox.Stat("middle.txt"); err != nil {
		t.Fatal("scan consumed input", err)
	}
	if err := inbox.Remove("middle.txt"); err != nil {
		t.Fatal(err)
	}
	claims, err := store.PendingInbox(t.Context(), "", 1)
	if err != nil || len(claims) != 1 || claims[0].ReceiptID != id {
		t.Fatalf("missing-file claim lost: %+v %v", claims, err)
	}
	claims, err = store.PendingInbox(t.Context(), claims[0].Name, 1)
	if err != nil || len(claims) != 0 {
		t.Fatalf("claim cursor: %+v %v", claims, err)
	}
}

func TestSettlingOneAcquisitionDoesNotRecoverActiveAnalysisOrConsumeInbox(t *testing.T) {
	store, _, home, managed := receiptStore(t)
	receipt := mustReceive(t, store)
	if _, err := home.DB().Exec(`UPDATE txt_files SET state=? WHERE id=?`, Receiving, receipt.ID); err != nil {
		t.Fatal(err)
	}
	other := mustReceive(t, store)
	if _, err := home.DB().Exec(`UPDATE txt_files SET state=? WHERE id=?`, Analyzing, other.ID); err != nil {
		t.Fatal(err)
	}
	value, err := store.SettleAcquisition(t.Context(), receipt.ID)
	if err != nil || value == nil || value.State != Received {
		t.Fatalf("settled: %+v %v", value, err)
	}
	current, err := store.Get(t.Context(), other.ID)
	if err != nil || current.State != Analyzing {
		t.Fatalf("unrelated analysis changed: %+v %v", current, err)
	}
	if _, err := managed.Stat(receipt.Path); err != nil {
		t.Fatal(err)
	}
	if err := store.Discard(t.Context(), receipt.ID); err != nil {
		t.Fatal(err)
	}
	if value, err := store.SettleAcquisition(t.Context(), receipt.ID); err != nil || value != nil {
		t.Fatalf("discarded receipt: %+v %v", value, err)
	}
	if _, err := managed.Stat(receipt.Path); !os.IsNotExist(err) {
		t.Fatalf("discarded original recreated: %v", err)
	}
}
