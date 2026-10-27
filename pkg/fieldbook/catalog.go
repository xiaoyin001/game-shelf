// 文件说明：保存三条有限探索故事与状态约束，世界和db共用同一内容白名单。
package fieldbook

import (
	"errors"
	"game-shelf/pkg/contracts"
)

// Stop 是可调查地点，Z为玩家站稳时的脚底高度。
type Stop struct {
	X, Y, Z float64
	Clue    string
}
type Entry struct {
	ID, Name, Story string
	Stops           [2]Stop
}

// Entries 返回独立值，不允许调用者改写其他世界的路线。
func Entries() [3]Entry {
	return [3]Entry{
		{ID: "bell", Name: "营灯旧铃", Story: "旧铃上刻着第一位守灯人的名字。那年浓雾封住归路，他把铃挂在营灯旁，让晚归的人听见家的方向。", Stops: [2]Stop{
			{600, 520, 0, "从阿苔身边往东走，营地东缘有一枚叶纹路标。靠近后按F调查。"},
			{628, 448, 8, "路标背面画着一块圆石：往北找营地东侧的矮石，旧铃就在石旁。"},
		}},
		{ID: "letter", Name: "湖畔来信", Story: "信纸被湖风吹得微卷：『我把回家的路画在水边，等你来时，就沿着芦苇慢慢走。』落款是一片干燥的苔叶。", Stops: [2]Stop{
			{790, 460, 0, "到原野东侧的湖边，先找西岸草地上的旧信封。沿营地东侧向东走，不必下水。"},
			{1015, 460, 0, "信封里只剩一片苔叶。绕到湖的东岸，与湖心齐平的草地上，另一封信还在等你。"},
		}},
		{ID: "flute", Name: "林间风笛", Story: "风笛没有簧片，只有风经过晶簇时才会鸣响。林哨说，愿意停下来听的人，都能记住属于自己的那段旋律。", Stops: [2]Stop{
			{365, 305, 0, "去营地西北，寻找西北晶簇东南侧的落叶记号。它在发光晶簇与营地之间。"},
			{300, 260, 50, "记号指向西北晶簇的顶端。按空格跳上晶簇，站稳后调查，才能听见风笛。"},
		}},
	}
}

func Next(j *contracts.Journal) string {
	entries := Entries()
	if len(j.Found) >= len(entries) {
		return ""
	}
	return entries[len(j.Found)].ID
}
func Current(j *contracts.Journal) *contracts.Trail {
	for _, e := range Entries() {
		if e.ID == j.Active && j.Step >= 0 && j.Step < int32(len(e.Stops)) {
			p := e.Stops[j.Step]
			return &contracts.Trail{ID: e.ID, Title: e.Name, Clue: p.Clue, X: p.X, Y: p.Y, Z: p.Z, Step: j.Step + 1, Total: int32(len(e.Stops))}
		}
	}
	return nil
}
func View(j *contracts.Journal) *contracts.FieldBook {
	book := &contracts.FieldBook{Trail: Current(j)}
	for i, e := range Entries() {
		found := i < len(j.Found)
		c := &contracts.Collection{ID: e.ID, Name: e.Name, Found: found}
		if found {
			c.Story = e.Story
		}
		book.Collections = append(book.Collections, c)
	}
	return book
}
func Clone(j *contracts.Journal) *contracts.Journal {
	if j == nil {
		return &contracts.Journal{}
	}
	copy := *j
	copy.Found = append([]string(nil), j.Found...)
	return &copy
}

// Validate 拒绝未知内容、重复/乱序收藏与损坏步骤，不把坏档当空档覆盖。
func Validate(j *contracts.Journal) error {
	if j == nil || j.Revision < 0 || j.Revision > 1000000 || len(j.Found) > len(Entries()) {
		return errors.New("invalid_field_book")
	}
	entries := Entries()
	for i, id := range j.Found {
		if id != entries[i].ID {
			return errors.New("invalid_field_book")
		}
	}
	if j.Active == "" {
		if j.Step != 0 {
			return errors.New("invalid_field_book")
		}
	} else if j.Active != Next(j) || j.Step < 0 || j.Step > 1 {
		return errors.New("invalid_field_book")
	}
	return nil
}
