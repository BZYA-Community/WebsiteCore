package media

// Only SPS syntax needed to determine decoded dimensions is read. All loops,
// Golomb values and allocations are bounded; this is not a video decoder.
type h264Bits struct {
	data    []byte
	pos     int
	invalid bool
}

func (b *h264Bits) bits(n int) uint32 {
	if b.invalid || n > 32 || b.pos+n > len(b.data)*8 {
		b.invalid = true
		return 0
	}
	var v uint32
	for range n {
		v = v<<1 | uint32(b.data[b.pos/8]>>(7-b.pos%8)&1)
		b.pos++
	}
	return v
}

func (b *h264Bits) ue(max uint32) uint32 {
	zeros := 0
	for !b.invalid && b.bits(1) == 0 {
		zeros++
		if zeros > 30 {
			b.invalid = true
		}
	}
	if b.invalid {
		return 0
	}
	v := uint32(1)<<zeros - 1 + b.bits(zeros)
	if v > max {
		b.invalid = true
		return 0
	}
	return v
}

func (b *h264Bits) se() int64 {
	v := int64(b.ue(1<<31 - 1))
	if v&1 == 1 {
		return (v + 1) / 2
	}
	return -v / 2
}

func h264Dimensions(nal []byte) (uint16, uint16, error) {
	if len(nal) < 5 || len(nal) > 65535 || nal[0]&0x9f != 7 || nal[0]&0x60 == 0 {
		return 0, 0, ErrInvalid
	}
	rbsp := make([]byte, 0, len(nal)-1)
	zeros := 0
	for i := 1; i < len(nal); i++ {
		v := nal[i]
		if zeros == 2 && v == 3 {
			if i+1 == len(nal) || nal[i+1] > 3 {
				return 0, 0, ErrInvalid
			}
			zeros = 0
			continue
		}
		if zeros == 2 && v < 3 {
			return 0, 0, ErrInvalid
		}
		rbsp = append(rbsp, v)
		if v == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	b := h264Bits{data: rbsp}
	profile := b.bits(8)
	if b.bits(8)&3 != 0 {
		return 0, 0, ErrInvalid
	}
	b.bits(8) // level_idc
	b.ue(31)  // seq_parameter_set_id
	chroma := uint32(1)
	separateColourPlane := uint32(0)
	switch profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134, 135:
		chroma = b.ue(3)
		if chroma == 3 {
			separateColourPlane = b.bits(1)
		}
		b.ue(6)   // bit_depth_luma_minus8
		b.ue(6)   // bit_depth_chroma_minus8
		b.bits(1) // qpprime_y_zero_transform_bypass_flag
		if b.bits(1) != 0 {
			count := 8
			if chroma == 3 {
				count = 12
			}
			for i := 0; i < count; i++ {
				if b.bits(1) == 0 {
					continue
				}
				size := 16
				if i >= 6 {
					size = 64
				}
				last, next := int64(8), int64(8)
				for range size {
					if next != 0 {
						next = (last + b.se()%256 + 256) % 256
					}
					if next != 0 {
						last = next
					}
				}
			}
		}
	case 66, 77, 88:
	default:
		return 0, 0, ErrInvalid
	}
	b.ue(12)         // log2_max_frame_num_minus4
	switch b.ue(2) { // pic_order_cnt_type
	case 0:
		b.ue(12)
	case 1:
		b.bits(1)
		b.se()
		b.se()
		count := b.ue(255)
		for range count {
			b.se()
		}
	}
	b.ue(16) // max_num_ref_frames
	b.bits(1)
	mbWidth, mapHeight := b.ue(119)+1, b.ue(119)+1
	frameOnly := b.bits(1)
	if frameOnly == 0 {
		b.bits(1)
	}
	b.bits(1)
	var left, right, top, bottom uint32
	if b.bits(1) != 0 {
		left, right, top, bottom = b.ue(1920), b.ue(1920), b.ue(1920), b.ue(1920)
	}
	// H.264 7.4.2.1.1: crop units depend on chroma and field coding.
	cropX, cropY := uint32(1), 2-frameOnly
	if separateColourPlane == 0 && chroma != 0 {
		if chroma != 3 {
			cropX = 2
		}
		if chroma == 1 {
			cropY *= 2
		}
	}
	codedW, codedH := mbWidth*16, mapHeight*16*(2-frameOnly)
	cropW, cropH := (left+right)*cropX, (top+bottom)*cropY
	// Allow macroblock padding (e.g. 1088 -> 1080), not arbitrarily large
	// coded frames hidden by cropping.
	if b.invalid || cropW >= codedW || cropH >= codedH ||
		max(codedW, codedH) > 1920 || min(codedW, codedH) > 1088 {
		return 0, 0, ErrInvalid
	}
	w, h := codedW-cropW, codedH-cropH
	if max(w, h) > 1920 || min(w, h) > 1080 {
		return 0, 0, ErrInvalid
	}
	return uint16(w), uint16(h), nil
}
