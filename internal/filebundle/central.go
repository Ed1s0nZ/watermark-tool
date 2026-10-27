package filebundle

import (
	"context"
	"encoding/binary"
	"io"
)

type directoryEnd struct {
	offset, size int64
	entries      int
}

func readAt(r io.ReaderAt, size int, offset int64) ([]byte, error) {
	buffer := make([]byte, size)
	if _, err := r.ReadAt(buffer, offset); err != nil {
		return nil, ErrArchive
	}
	return buffer, nil
}

// inspectCentral runs before archive/zip allocates its entry table.
func inspectCentral(c context.Context, r io.ReaderAt, size int64, limits Limits) (directoryEnd, error) {
	if !limits.Valid() {
		return directoryEnd{}, ErrLimit
	}
	if size < 22 {
		return directoryEnd{}, ErrArchive
	}
	if size > limits.MaxArchiveBytes {
		return directoryEnd{}, ErrLimit
	}
	length := int64(22 + 65535)
	if length > size {
		length = size
	}
	tail, err := readAt(r, int(length), size-length)
	if err != nil {
		return directoryEnd{}, err
	}
	end := -1
	for i := len(tail) - 22; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:i+4]) == 0x06054b50 && i+22+int(binary.LittleEndian.Uint16(tail[i+20:i+22])) == len(tail) {
			end = i
			break
		}
	}
	if end < 0 {
		return directoryEnd{}, ErrArchive
	}
	e := tail[end:]
	disk := binary.LittleEndian.Uint16(e[4:6])
	centralDisk := binary.LittleEndian.Uint16(e[6:8])
	diskEntries := binary.LittleEndian.Uint16(e[8:10])
	entries := binary.LittleEndian.Uint16(e[10:12])
	centralSize := binary.LittleEndian.Uint32(e[12:16])
	offset := binary.LittleEndian.Uint32(e[16:20])
	position := size - length + int64(end)
	if disk != 0 || centralDisk != 0 || diskEntries != entries || entries == 0xffff || centralSize == 0xffffffff || offset == 0xffffffff {
		return directoryEnd{}, ErrArchive
	}
	if int(entries) > limits.MaxEntries || int64(centralSize) > limits.MaxCentralBytes {
		return directoryEnd{}, ErrLimit
	}
	if int64(offset)+int64(centralSize) != position {
		return directoryEnd{}, ErrArchive
	}
	if entries == 0 {
		return directoryEnd{}, ErrEmpty
	}
	signature, err := readAt(r, 4, 0)
	if err != nil || binary.LittleEndian.Uint32(signature) != 0x04034b50 {
		return directoryEnd{}, ErrArchive
	}
	cursor := int64(offset)
	boundary := cursor + int64(centralSize)
	for entry := 0; entry < int(entries); entry++ {
		if err = c.Err(); err != nil {
			return directoryEnd{}, err
		}
		if cursor+46 > boundary {
			return directoryEnd{}, ErrArchive
		}
		header, err := readAt(r, 46, cursor)
		if err != nil || binary.LittleEndian.Uint32(header) != 0x02014b50 {
			return directoryEnd{}, ErrArchive
		}
		flags := binary.LittleEndian.Uint16(header[8:10])
		method := binary.LittleEndian.Uint16(header[10:12])
		nameLength := int(binary.LittleEndian.Uint16(header[28:30]))
		extraLength := int(binary.LittleEndian.Uint16(header[30:32]))
		commentLength := int(binary.LittleEndian.Uint16(header[32:34]))
		if flags&(1|64|0x2000) != 0 || (method != 0 && method != 8) || binary.LittleEndian.Uint16(header[34:36]) != 0 || binary.LittleEndian.Uint32(header[20:24]) == 0xffffffff || binary.LittleEndian.Uint32(header[24:28]) == 0xffffffff || binary.LittleEndian.Uint32(header[42:46]) == 0xffffffff {
			return directoryEnd{}, ErrArchive
		}
		if nameLength < 1 || nameLength > 513 {
			return directoryEnd{}, ErrPath
		}
		next := cursor + 46 + int64(nameLength+extraLength+commentLength)
		if next > boundary {
			return directoryEnd{}, ErrArchive
		}
		extra, err := readAt(r, extraLength, cursor+46+int64(nameLength))
		if err != nil {
			return directoryEnd{}, err
		}
		for i := 0; i < len(extra); {
			if i+4 > len(extra) {
				return directoryEnd{}, ErrArchive
			}
			id := binary.LittleEndian.Uint16(extra[i : i+2])
			n := int(binary.LittleEndian.Uint16(extra[i+2 : i+4]))
			if id == 1 || i+4+n > len(extra) {
				return directoryEnd{}, ErrArchive
			}
			i += 4 + n
		}
		cursor = next
	}
	if cursor != boundary {
		return directoryEnd{}, ErrArchive
	}
	return directoryEnd{offset: int64(offset), size: int64(centralSize), entries: int(entries)}, nil
}
