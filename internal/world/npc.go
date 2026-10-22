// 文件说明：在世界锁内校验 NPC 交互与本轮委托，私有对话不进入广播快照。
package world

import (
	"fmt"
	"game-shelf/pkg/contracts"
	"game-shelf/pkg/scene"
	"math"
	"time"
)

const (
	questGoal   = 2
	questReward = 3
)

func (w *World) response(p *player, now time.Time, dialog *contracts.WorldDialog) *contracts.WorldResp {
	quest := p.quest
	quest.Goal, quest.Reward = questGoal, questReward
	return &contracts.WorldResp{State: w.snapshot(now), Quest: &quest, Dialog: dialog}
}

func (w *World) interact(p *player, req *contracts.WorldReq) (*contracts.WorldDialog, string) {
	var npc *scene.NPC
	for i := range w.layout.NPCs {
		if w.layout.NPCs[i].ID == req.NPCID {
			npc = &w.layout.NPCs[i]
			break
		}
	}
	if npc == nil {
		return nil, "unknown_npc"
	}
	if math.Hypot(math.Hypot(p.state.X-npc.X, p.state.Y-npc.Y), p.state.Z) > npc.Range {
		return nil, "npc_out_of_range"
	}
	dialog := &contracts.WorldDialog{NPCID: npc.ID, Name: npc.Role + " · " + npc.Name}
	choice := req.ChoiceID
	if req.Action == "interact" && choice != "" {
		return nil, "invalid_npc_choice"
	}
	if req.Action == "choose" && choice == "" {
		return nil, "invalid_npc_choice"
	}
	add := func(id, label string) {
		dialog.Choices = append(dialog.Choices, &contracts.DialogChoice{ID: id, Label: label})
	}
	if npc.ID == "ranger" {
		switch choice {
		case "":
			dialog.Text = "我是林哨，在林间巡看。想了解营地，还是发光的晶簇？"
		case "camp":
			dialog.Text = "守灯人阿苔在营地南侧（540 / 500）。他正在为营灯寻找微光，去和他聊聊吧。低矮岩石可以跳上去，树木与帐篷会挡住去路。"
		case "crystals":
			dialog.Text = "五处晶簇散落在原野。最近的一处在 620 / 400；靠近按 E 采集，同一晶簇由所有旅人共享，采过后要等 10 秒。你可以等它再亮，也可以探索别处。"
		default:
			return nil, "invalid_npc_choice"
		}
		add("camp", "营地里有什么？")
		add("crystals", "哪里能找到晶簇？")
		return dialog, ""
	}
	// 状态转换与采集/加分同持一把锁，重复或并发提交不能再次奖励。
	switch choice {
	case "":
	case "advice":
	case "accept":
		if p.quest.Status == "" {
			p.quest.Status = "active"
		}
	case "complete":
		if p.quest.Status == "completed" {
			break
		}
		if p.quest.Status != "active" || p.quest.Progress < questGoal {
			return nil, "quest_not_ready"
		}
		p.quest.Status = "completed"
		p.state.Score += questReward
	default:
		return nil, "invalid_npc_choice"
	}
	switch p.quest.Status {
	case "":
		dialog.Text = "欢迎来到苔光原野！我在为营灯收集微光。愿意帮忙吗？接取后采集两次晶簇，再回来找我，额外奖励 3 分微光。"
		add("accept", "接取委托：点亮营灯")
	case "active":
		dialog.Text = fmt.Sprintf("点亮营灯：已采集 %d / %d。接取前的采集不计入委托。", p.quest.Progress, questGoal)
		if p.quest.Progress >= questGoal {
			dialog.Text += "微光已经足够，谢谢你！可以领取额外 3 分奖励了。"
			add("complete", "完成委托 · 领取 3 分微光")
		}
	case "completed":
		dialog.Text = "营灯因你更明亮了。委托已完成，额外 3 分微光已经计入本轮分数，不可重复领取。也去听听巡林员林哨的故事吧！"
	}
	if choice == "advice" {
		dialog.Text = "最近的晶簇在 620 / 400；靠近后按 E 采集。采过后等 10 秒再亮，或探索其他晶簇。委托只统计接取后成功的采集，回到我身边才能领奖。"
	}
	add("advice", "告诉我采集方法")
	return dialog, ""
}
