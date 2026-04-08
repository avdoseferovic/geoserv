package pub

import (
	"fmt"
	"os"
	"sync"

	"github.com/ethanmoffat/eolib-go/v3/data"
	eopubsrv "github.com/ethanmoffat/eolib-go/v3/protocol/pub/server"
)

var (
	InnDB          *eopubsrv.InnFile
	ShopFileDB     *eopubsrv.ShopFile
	SkillMasterDB  *eopubsrv.SkillMasterFile
	serverDataPath struct {
		inn   string
		drop  string
		talk  string
		shop  string
		skill string
	}

	saveMu sync.Mutex
)

func LoadServerData() {
	serverDataPath.inn = "data/pub/din001.eid"
	serverDataPath.drop = "data/pub/dtd001.edf"
	serverDataPath.talk = "data/pub/ttd001.etf"
	serverDataPath.shop = "data/pub/dts001.esf"
	serverDataPath.skill = "data/pub/dsm001.emf"

	if inn, err := loadInnFile(serverDataPath.inn); err == nil {
		InnDB = inn
	} else {
		InnDB = &eopubsrv.InnFile{}
	}

	if shop, err := loadShopFile(serverDataPath.shop); err == nil {
		ShopFileDB = shop
	} else {
		ShopFileDB = &eopubsrv.ShopFile{}
	}

	if skill, err := loadSkillMasterFile(serverDataPath.skill); err == nil {
		SkillMasterDB = skill
	} else {
		SkillMasterDB = &eopubsrv.SkillMasterFile{}
	}
}

func loadInnFile(path string) (*eopubsrv.InnFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := data.NewEoReader(raw)
	var f eopubsrv.InnFile
	if err := f.Deserialize(reader); err != nil {
		return nil, err
	}
	return &f, nil
}

func loadShopFile(path string) (*eopubsrv.ShopFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := data.NewEoReader(raw)
	var f eopubsrv.ShopFile
	if err := f.Deserialize(reader); err != nil {
		return nil, err
	}
	return &f, nil
}

func loadSkillMasterFile(path string) (*eopubsrv.SkillMasterFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := data.NewEoReader(raw)
	var f eopubsrv.SkillMasterFile
	if err := f.Deserialize(reader); err != nil {
		return nil, err
	}
	return &f, nil
}

func serializeToBytes(s interface{ Serialize(*data.EoWriter) error }) ([]byte, error) {
	writer := data.NewEoWriter()
	if err := s.Serialize(writer); err != nil {
		return nil, err
	}
	return writer.Array(), nil
}

func saveFile(path string, s interface{ Serialize(*data.EoWriter) error }) error {
	raw, err := serializeToBytes(s)
	if err != nil {
		return fmt.Errorf("serializing: %w", err)
	}
	return os.WriteFile(path, raw, 0o644)
}

func SaveDrops(df *eopubsrv.DropFile) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	if err := saveFile(serverDataPath.drop, df); err != nil {
		return err
	}
	DropDB = df
	npcDropIndex = make(map[int][]eopubsrv.DropRecord, len(df.Npcs))
	for _, npc := range df.Npcs {
		npcDropIndex[npc.NpcId] = npc.Drops
	}
	return nil
}

func SaveTalk(tf *eopubsrv.TalkFile) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	if err := saveFile(serverDataPath.talk, tf); err != nil {
		return err
	}
	TalkDB = tf
	npcTalkIndex = make(map[int]eopubsrv.TalkRecord, len(tf.Npcs))
	for _, npc := range tf.Npcs {
		npcTalkIndex[npc.NpcId] = npc
	}
	return nil
}

func SaveInns(f *eopubsrv.InnFile) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	if err := saveFile(serverDataPath.inn, f); err != nil {
		return err
	}
	InnDB = f
	return nil
}

func SaveShops(f *eopubsrv.ShopFile) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	if err := saveFile(serverDataPath.shop, f); err != nil {
		return err
	}
	ShopFileDB = f
	return nil
}

func SaveSkillMasters(f *eopubsrv.SkillMasterFile) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	if err := saveFile(serverDataPath.skill, f); err != nil {
		return err
	}
	SkillMasterDB = f
	return nil
}
