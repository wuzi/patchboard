package uploads

// HoldDecodeSlots occupies every image-decoding slot until release is called.
func HoldDecodeSlots() (release func()) {
	for i := 0; i < cap(decodeSlots); i++ {
		decodeSlots <- struct{}{}
	}
	return func() {
		for i := 0; i < cap(decodeSlots); i++ {
			<-decodeSlots
		}
	}
}
