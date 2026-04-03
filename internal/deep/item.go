package deep

import "github.com/ethanmoffat/eolib-go/v3/data"

type ItemReportClientPacket struct {
	ItemID int
	Title  string
}

func DeserializeItemReport(reader *data.EoReader) (ItemReportClientPacket, error) {
	wasChunked := reader.IsChunked()
	defer reader.SetIsChunked(wasChunked)

	reader.SetIsChunked(true)

	itemID := reader.GetShort()
	if err := reader.NextChunk(); err != nil {
		return ItemReportClientPacket{}, err
	}
	title, err := reader.GetString()
	if err != nil {
		return ItemReportClientPacket{}, err
	}

	return ItemReportClientPacket{ItemID: itemID, Title: title}, nil
}
