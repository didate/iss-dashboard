package api

import (
	"bytes"
	"compress/gzip"
	"testing"
)

// Le CSV annuel dépasse la limite d'envoi par défaut des reverse proxies :
// l'import doit accepter sa version gzippée aussi bien que le fichier brut.
func TestGunzipIfNeeded(t *testing.T) {
	plain := []byte("region;prefecture\nBOKE;Boké\n")

	got, err := gunzipIfNeeded(plain)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("un CSV non compressé doit passer tel quel : %q (err %v)", got, err)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write(plain)
	zw.Close()
	got, err = gunzipIfNeeded(buf.Bytes())
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("un CSV gzippé doit être décompressé : %q (err %v)", got, err)
	}

	if _, err := gunzipIfNeeded([]byte{0x1f, 0x8b, 0x00, 0x01}); err == nil {
		t.Fatal("un faux gzip doit remonter une erreur, pas passer silencieusement")
	}
	if got, err := gunzipIfNeeded(nil); err != nil || len(got) != 0 {
		t.Fatalf("entrée vide : %q (err %v)", got, err)
	}
}
