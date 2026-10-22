// 文件说明：真实双连接只发移动意图，验证现有低石顶面、对端高度、停留与走离落地。
package integration

import (
	"game-shelf/pkg/scene"
	"testing"
)

func TestLiveWalkOverLowStoneObservedByPeer(t *testing.T) {
	live(t)
	first, second := login(t, name()), login(t, name())
	a, b := auth(t, first), auth(t, second)
	var rock scene.Entity
	for _, e := range scene.Default().Entities {
		if e.ID == "rock-4" {
			rock = e
		}
	}
	if rock.Height != 8 {
		t.Fatal("missing canonical low stone")
	}
	// 旁观者离开通道，不能把玩家身体的阻挡误当成石头阻挡。
	walkNPC(t, b, second.UserID, 680, 320)
	walkNPC(t, a, first.UserID, 560, rock.Y+12)
	stood := false
	for range 15 {
		send(t, a, "input", map[string]any{"dx": 1})
		p := findPlayer(t, read(t, a, "state").State, first.UserID)
		if p.Z == rock.Height && p.Grounded {
			stood = true
			break
		}
	}
	if !stood {
		t.Fatal("walking did not step onto real rock")
	}
	send(t, a, "input", nil)
	observed := false
	for range 50 {
		p := findPlayer(t, read(t, b, "state").State, first.UserID)
		if p.Z == rock.Height && p.Grounded {
			observed = true
			break
		}
	}
	if !observed {
		t.Fatal("peer did not observe standing on rock")
	}
	for range 5 {
		p := findPlayer(t, read(t, a, "state").State, first.UserID)
		if p.Z != rock.Height || !p.Grounded {
			t.Fatal("idle player did not remain on top", p)
		}
	}
	landed := false
	for range 16 {
		send(t, a, "input", map[string]any{"dx": 1})
		p := findPlayer(t, read(t, a, "state").State, first.UserID)
		if p.X > rock.X+rock.Radius+20 && p.Z == 0 && p.Grounded {
			landed = true
			break
		}
	}
	if !landed {
		t.Fatal("did not walk across and fall back to ground")
	}
}
