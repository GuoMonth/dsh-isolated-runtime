package cellcontract

import "testing"

func TestResourceContract(t *testing.T) {
	t.Parallel()
	const uid = "12345678-1234-1234-1234-123456789abc"
	names := ResourceNames(uid)
	if names.Base != "cell-"+uid || names.DataPVC != names.Base+"-data" || names.PrivatePVC != names.Base+"-private" || names.Headless != names.Base+"-headless" {
		t.Fatalf("unexpected names: %#v", names)
	}
	if got := Authority("tenant-a", uid); got != names.Base+".tenant-a.svc" {
		t.Fatalf("authority = %q", got)
	}
	if got := SnapshotName(uid); got != "cellsnapshot-"+uid {
		t.Fatalf("snapshot name = %q", got)
	}
}
