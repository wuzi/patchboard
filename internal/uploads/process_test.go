package uploads_test

import (
	"bytes"
	"errors"
	"image"
	"testing"
	"time"

	"patchboard/internal/testutil"
	"patchboard/internal/uploads"
)

func TestProcess_StripsMetadata(t *testing.T) {
	in := jpegWithExif(t)
	if !bytes.Contains(in, []byte("Exif")) {
		t.Fatal("fixture lacks Exif")
	}
	out, ext, w, h, err := uploads.Process(bytes.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Eq(t, ext, "jpg")
	testutil.Eq(t, w, 40)
	testutil.Eq(t, h, 30)
	if bytes.Contains(out, []byte("Exif")) || bytes.Contains(out, []byte("GPS-SECRET")) {
		t.Fatal("metadata survived re-encoding")
	}
}

func TestProcess_Resizes(t *testing.T) {
	out, ext, w, h, err := uploads.Process(bytes.NewReader(pngBytes(t, 3000, 1000)))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Eq(t, ext, "png")
	testutil.Eq(t, w, 2560)
	testutil.Eq(t, h, 853)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 2560 || cfg.Height != 853 {
		t.Fatalf("output config = %+v %v", cfg, err)
	}
}

func TestProcess_GIFBecomesPNG(t *testing.T) {
	_, ext, _, _, err := uploads.Process(bytes.NewReader(gifBytes(t)))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Eq(t, ext, "png")
}

func TestProcess_RejectsNonImage(t *testing.T) {
	_, _, _, _, err := uploads.Process(bytes.NewReader([]byte("hello")))
	if !errors.Is(err, uploads.ErrNotImage) {
		t.Fatalf("err = %v", err)
	}
}

func TestProcess_RejectsTooManyPixels(t *testing.T) {
	_, _, _, _, err := uploads.Process(bytes.NewReader(hugePNG(t, 20000, 20000)))
	if !errors.Is(err, uploads.ErrTooManyPixels) {
		t.Fatalf("err = %v", err)
	}
}

func TestProcess_WaitsForADecodeSlot(t *testing.T) {
	release := uploads.HoldDecodeSlots()
	done := make(chan error, 1)
	go func() {
		_, _, _, _, err := uploads.Process(bytes.NewReader(pngBytes(t, 8, 8)))
		done <- err
	}()
	select {
	case <-done:
		release()
		t.Fatal("Process decoded while every slot was taken")
	case <-time.After(150 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Process never got a slot")
	}
}
