// 文件说明：用共享场景和真实 WebGL2 呈现权威三维位置；镜头与插值不计算物理结果。
import * as THREE from "./vendor/three.module.js";

const clamp = THREE.MathUtils.clamp;
const color = (value) => new THREE.MeshLambertMaterial({ color: value, flatShading: true });

// WorldView 持有 GPU 资源、角色和镜头；网络与账号状态仍由 app 管理。
export class WorldView {
  constructor(canvas, onFailure, layout) {
    this.layout = layout;
    canvas.dataset.sceneVersion = layout.version;
    this.canvas = canvas;
    this.failed = false;
    this.onFailure = onFailure;
    this.players = new Map();
    this.nodes = new Map();
    this.clue = null;
    this.campProjection = null;
    this.yaw = 0;
    this.pitch = 0.65;
    this.distance = 560;
    this.target = new THREE.Vector3(560, 16, 400);
    this.lastTime = 0;
    this.activityUntil = 0;
    this.renderedFrames = 0;
    this.renderer = new THREE.WebGLRenderer({
      canvas,
      antialias: true,
      powerPreference: "low-power",
    });
    const gl = this.renderer.getContext();
    if (!(gl instanceof WebGL2RenderingContext)) throw new Error("需要 WebGL2 支持");
    canvas.dataset.renderer = "webgl2";
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 1.5));
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFSoftShadowMap;
    // 静态景物不逐帧重画阴影；角色变化最多每 100ms 刷新一次，保留真实深度与投影。
    this.renderer.shadowMap.autoUpdate = false;
    this.renderer.shadowMap.needsUpdate = true;
    this.shadowDirty = true;
    this.lastShadow = 0;
    this.renderer.toneMapping = THREE.ACESFilmicToneMapping;
    this.renderer.toneMappingExposure = 1.3;
    this.scene = new THREE.Scene();
    this.scene.background = new THREE.Color(0x8fa9a6);
    this.scene.fog = new THREE.Fog(0x8fa9a6, 950, 2200);
    this.camera = new THREE.PerspectiveCamera(48, 1, 1, 4000);
    this.camera.position.set(560, 450, 960);
    this.scene.add(new THREE.HemisphereLight(0xd0eee9, 0x293f29, 1.8));
    const sun = new THREE.DirectionalLight(0xffe9c5, 2.0);
    sun.position.set(260, 750, 180);
    sun.target.position.set(600, 0, 400);
    sun.castShadow = true;
    sun.shadow.mapSize.set(1024, 1024);
    Object.assign(sun.shadow.camera, { left: -800, right: 800, top: 700, bottom: -700, far: 1800 });
    sun.shadow.normalBias = 0.7;
    this.scene.add(sun, sun.target);
    this.makeLandscape();
    this.makeAvatarBatches();
    this.npcs = layout.npcs.map((npc) => {
      const entry = this.makePlayer(
        { ...npc, z: 0, name: npc.role + " · " + npc.name + " [E]" },
        false,
        true,
      );
      entry.group.rotation.y = npc.id === "keeper" ? Math.PI : 0;
      entry.label.position.set(npc.x, 88, npc.y);
      return entry;
    });
    this.makeControls();
    this.observer = new ResizeObserver(() => this.resize());
    this.observer.observe(canvas.parentElement);
    this.resize();
    canvas.addEventListener("webglcontextlost", (event) => {
      event.preventDefault();
      this.failed = true;
      onFailure("3D 上下文已丢失，请刷新页面；不会使用 2D 替代。");
    });
  }

  resize() {
    this.activityUntil = performance.now() + 750;
    const bounds = this.canvas.parentElement.getBoundingClientRect();
    const width = Math.max(1, Math.round(bounds.width));
    const height = Math.max(1, Math.round(bounds.height));
    this.renderer.setSize(width, height, false);
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
  }

  makeControls() {
    let drag = null;
    this.canvas.addEventListener("contextmenu", (event) => event.preventDefault());
    this.canvas.addEventListener("pointerdown", (event) => {
      if (event.button !== 2) return;
      drag = { x: event.clientX, y: event.clientY };
      this.activityUntil = performance.now() + 750;
      this.canvas.setPointerCapture(event.pointerId);
      this.canvas.classList.add("orbiting");
    });
    this.canvas.addEventListener("pointermove", (event) => {
      if (!drag) return;
      this.activityUntil = performance.now() + 750;
      this.yaw -= (event.clientX - drag.x) * 0.008;
      this.pitch = clamp(this.pitch + (event.clientY - drag.y) * 0.004, 0.4, 1.1);
      drag = { x: event.clientX, y: event.clientY };
    });
    const stop = () => {
      drag = null;
      this.canvas.classList.remove("orbiting");
    };
    this.canvas.addEventListener("pointerup", stop);
    this.canvas.addEventListener("pointercancel", stop);
    this.canvas.addEventListener("lostpointercapture", stop);
    window.addEventListener("blur", stop);
    this.canvas.addEventListener(
      "wheel",
      (event) => {
        event.preventDefault();
        this.distance = clamp(this.distance + event.deltaY * 0.4, 280, 1000);
        this.activityUntil = performance.now() + 750;
      },
      { passive: false },
    );
  }

  resetCamera() {
    this.activityUntil = performance.now() + 750;
    this.yaw = 0;
    this.pitch = 0.65;
    this.distance = 560;
  }

  // input 只把按键方向转到镜头对应的 X/Z 平面，速度和最终坐标由服务器决定。
  input(dx, dy) {
    const length = Math.max(1, Math.hypot(dx, dy));
    const c = Math.cos(this.yaw),
      s = Math.sin(this.yaw);
    return { dx: (dx * c + dy * s) / length, dy: (-dx * s + dy * c) / length };
  }

  setCanvasData(key, value) {
    const text = String(value);
    if (this.canvas.dataset[key] !== text) this.canvas.dataset[key] = text;
  }

  addMesh(geometry, material, x, y, z, parent = this.scene) {
    const mesh = new THREE.Mesh(geometry, material);
    mesh.position.set(x, y, z);
    mesh.castShadow = true;
    mesh.receiveShadow = true;
    parent.add(mesh);
    return mesh;
  }

  makeLandscape() {
    const ground = new THREE.PlaneGeometry(1600, 1200, 40, 30);
    ground.rotateX(-Math.PI / 2);
    const positions = ground.getAttribute("position"),
      colors = [];
    for (let i = 0; i < positions.count; i++) {
      const tint = new THREE.Color().setHSL(
        0.29 + Math.sin(i * 13.7) * 0.02,
        0.23,
        0.29 + Math.sin(i * 31.4) * 0.025,
      );
      colors.push(tint.r, tint.g, tint.b);
    }
    ground.setAttribute("color", new THREE.Float32BufferAttribute(colors, 3));
    const land = this.addMesh(
      ground,
      new THREE.MeshLambertMaterial({ vertexColors: true, flatShading: true }),
      600,
      -1,
      400,
    );
    land.castShadow = false;
    this.makePath();
    const water = this.addMesh(
      new THREE.CylinderGeometry(92, 100, 2, 18),
      new THREE.MeshPhongMaterial({ color: 0x386b70, shininess: 90 }),
      930,
      0,
      465,
    );
    water.scale.z = 0.65;
    const shore = this.addMesh(
      new THREE.CylinderGeometry(107, 115, 1, 18),
      color(0x6b7860),
      930,
      -0.3,
      465,
    );
    shore.scale.z = 0.65;
    const trees = this.layout.entities.filter((e) => e.kind === "tree");
    const dummy = new THREE.Object3D();
    const trunk = new THREE.InstancedMesh(
      new THREE.CylinderGeometry(5, 8, 38, 6),
      color(0x665543),
      trees.length,
    );
    const lower = new THREE.InstancedMesh(
      new THREE.ConeGeometry(33, 62, 7),
      color(0x315d43),
      trees.length,
    );
    const upper = new THREE.InstancedMesh(
      new THREE.ConeGeometry(26, 54, 7),
      color(0x4d7754),
      trees.length,
    );
    trees.forEach(({ x, y: z, scale }, i) => {
      dummy.scale.setScalar(scale);
      for (const [mesh, y] of [
        [trunk, 19],
        [lower, 60],
        [upper, 87],
      ]) {
        dummy.position.set(x, y * scale, z);
        dummy.rotation.set(0, i * 0.7, 0);
        dummy.updateMatrix();
        mesh.setMatrixAt(i, dummy.matrix);
      }
    });
    for (const mesh of [trunk, lower, upper]) {
      mesh.castShadow = true;
      mesh.receiveShadow = true;
      mesh.computeBoundingSphere();
      this.scene.add(mesh);
    }
    const wood = color(0x755744);
    const camp = this.layout.entities.find((e) => e.kind === "campfire");
    const shelter = this.layout.entities.find((e) => e.kind === "tent");
    // 同一几何/材质的静态石块合批，实例矩阵保持原坐标、半径和扁平比例。
    const rocks = this.layout.entities
      .filter((e) => e.kind === "rock")
      .map((e) => ({
        x: e.x,
        y: 3,
        z: e.y,
        radius: e.radius,
        flat: 0.5,
      }));
    for (let i = 0; i < 10; i++) {
      const a = (i / 10) * Math.PI * 2;
      rocks.push({
        x: camp.x + Math.cos(a) * 31,
        y: 4,
        z: camp.y + Math.sin(a) * 31,
        radius: 7,
        flat: 1,
      });
    }
    const stones = new THREE.InstancedMesh(
      new THREE.DodecahedronGeometry(1, 0),
      color(0x859184),
      rocks.length,
    );
    rocks.forEach((rock, i) => {
      dummy.position.set(rock.x, rock.y, rock.z);
      dummy.rotation.set(0, 0, 0);
      dummy.scale.set(rock.radius, rock.radius * rock.flat, rock.radius);
      dummy.updateMatrix();
      stones.setMatrixAt(i, dummy.matrix);
    });
    stones.castShadow = true;
    stones.receiveShadow = true;
    stones.computeBoundingSphere();
    this.scene.add(stones);
    this.canvas.dataset.stoneInstances = String(stones.count);
    this.canvas.dataset.stoneBatches = String(stones.isInstancedMesh ? 1 : 0);
    for (let i = 0; i < 3; i++) {
      const log = this.addMesh(
        new THREE.CylinderGeometry(4, 5, 38, 6),
        wood,
        camp.x,
        5 + i * 2,
        camp.y,
      );
      log.rotation.z = Math.PI / 2;
      log.rotation.y = (i * Math.PI) / 3;
    }
    this.flame = this.addMesh(
      new THREE.ConeGeometry(13, 39, 6),
      new THREE.MeshBasicMaterial({ color: 0xffc16b }),
      camp.x,
      25,
      camp.y,
    );
    this.fire = new THREE.PointLight(0xffac52, 5000, 180, 2);
    this.fire.position.set(camp.x, 42, camp.y);
    this.scene.add(this.fire);
    const tent = this.addMesh(
      new THREE.ConeGeometry(Math.SQRT2 * shelter.half_x, shelter.height, 4),
      color(0xc8a171),
      shelter.x,
      shelter.height / 2,
      shelter.y,
    );
    tent.rotation.y = Math.PI / 4;
    this.addMesh(new THREE.BoxGeometry(15, 34, 2), color(0x3c493d), shelter.x, 17, shelter.y + 35);
    this.geometries = {
      body: new THREE.CylinderGeometry(12, 15, 25, 7),
      head: new THREE.SphereGeometry(8, 10, 7),
      eye: new THREE.SphereGeometry(1.4, 6, 4),
      cap: new THREE.ConeGeometry(15, 16, 7),
      limb: new THREE.BoxGeometry(7, 15, 9),
      bag: new THREE.BoxGeometry(17, 19, 8),
    };
  }

  makePath() {
    const curve = new THREE.CatmullRomCurve3([
      new THREE.Vector3(535, 0.3, 850),
      new THREE.Vector3(500, 0.3, 620),
      new THREE.Vector3(620, 0.3, 400),
      new THREE.Vector3(560, 0.3, 200),
      new THREE.Vector3(650, 0.3, -30),
    ]);
    const points = [],
      indices = [];
    for (let i = 0; i <= 64; i++) {
      const t = i / 64,
        p = curve.getPoint(t),
        d = curve.getTangent(t);
      points.push(p.x - d.z * 27, p.y, p.z + d.x * 27, p.x + d.z * 27, p.y, p.z - d.x * 27);
      if (i < 64) {
        const a = i * 2;
        indices.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
      }
    }
    const geometry = new THREE.BufferGeometry();
    geometry.setAttribute("position", new THREE.Float32BufferAttribute(points, 3));
    geometry.setIndex(indices);
    geometry.computeVertexNormals();
    const path = this.addMesh(
      geometry,
      new THREE.MeshLambertMaterial({ color: 0x84795b, side: THREE.DoubleSide }),
      0,
      0,
      0,
    );
    path.castShadow = false;
  }

  label(text, tint) {
    const canvas = document.createElement("canvas");
    canvas.width = 384;
    canvas.height = 80;
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "rgba(14,35,39,0.85)";
    ctx.beginPath();
    ctx.roundRect(2, 2, 380, 76, 14);
    ctx.fill();
    ctx.font = "600 25px 'Microsoft YaHei','Segoe UI',sans-serif";
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.fillStyle = tint;
    ctx.fillText(text, 192, 40, 350);
    const texture = new THREE.CanvasTexture(canvas);
    texture.colorSpace = THREE.SRGBColorSpace;
    const sprite = new THREE.Sprite(
      new THREE.SpriteMaterial({ map: texture, transparent: true, depthWrite: false }),
    );
    sprite.scale.set(132, 27.5, 1);
    return sprite;
  }

  makeAvatarBatches() {
    this.avatarBatches = {};
    this.avatarProbe = new THREE.Matrix4();
    // 容量对应当前服务器的100位玩家与固定NPC，实际绘制数每帧单独设置。
    const capacity = 100 + this.layout.npcs.length;
    for (const kind of ["body", "head", "eye", "cap", "limb", "bag", "ring"]) {
      const geometry = kind === "ring" ? new THREE.RingGeometry(21, 23, 32) : this.geometries[kind];
      const material =
        kind === "ring"
          ? new THREE.MeshBasicMaterial({
              color: 0xffffff,
              side: THREE.DoubleSide,
              transparent: true,
              opacity: 0.8,
            })
          : color(0xffffff);
      const count = capacity * (kind === "limb" ? 4 : kind === "eye" ? 2 : 1);
      const mesh = new THREE.InstancedMesh(geometry, material, count);
      mesh.count = 0;
      mesh.castShadow = mesh.receiveShadow = kind !== "ring";
      // 动态角色不能按上一帧的静态包围球裁掉；实际实例仍受GPU视锥/深度裁剪。
      mesh.frustumCulled = false;
      this.scene.add(mesh);
      this.avatarBatches[kind] = mesh;
    }
  }

  makePlayer(player, own, npc = false) {
    const group = new THREE.Group();
    group.position.set(player.x, player.z, player.y);
    group.name = (npc ? "npc:" : "traveller:") + player.id;
    const coat = npc ? (player.id === "keeper" ? 0xd9b468 : 0x65b9b5) : own ? 0x9fd4b5 : 0x9faed3;
    const hat = npc ? 0x81643e : own ? 0x587951 : 0x656d99;
    const parts = [];
    const piece = (kind, tint, x, y, z) => {
      const anchor = new THREE.Object3D();
      anchor.position.set(x, y, z);
      anchor.userData.kind = kind;
      anchor.userData.tint = new THREE.Color(tint);
      group.add(anchor);
      parts.push(anchor);
      return anchor;
    };
    piece("body", coat, 0, 25, 0);
    piece("head", 0xf0d3b0, 0, 45, 0);
    piece("cap", hat, 0, 57, 0);
    piece("bag", 0x826b4d, 0, 26, -11);
    piece("eye", 0x465351, -3, 46, 7);
    piece("eye", 0x465351, 3, 46, 7);
    const legs = [piece("limb", 0x465351, -6, 8, 0), piece("limb", 0x465351, 6, 8, 0)];
    piece("limb", coat, -17, 26, 0);
    piece("limb", coat, 17, 26, 0);
    if (own || npc) {
      const ring = piece("ring", npc ? 0xffd77c : 0xc8f1bd, 0, 0.6, 0);
      ring.rotation.x = -Math.PI / 2;
    }
    const label = this.label(
      player.name + (own ? " · 你" : ""),
      npc ? "#ffe4a6" : own ? "#d6ffdc" : "#e3e8ff",
    );
    label.position.y = 82;
    this.scene.add(group, label);
    return { group, label, legs, parts, target: new THREE.Vector3(player.x, player.z, player.y) };
  }

  drawAvatars() {
    const counts = Object.fromEntries(Object.keys(this.avatarBatches).map((kind) => [kind, 0]));
    for (const entry of [...this.npcs, ...this.players.values()]) {
      entry.group.updateMatrixWorld(true);
      for (const part of entry.parts) {
        const kind = part.userData.kind,
          index = counts[kind]++;
        const mesh = this.avatarBatches[kind];
        mesh.setMatrixAt(index, part.matrixWorld);
        mesh.setColorAt(index, part.userData.tint);
        if (kind === "body") entry.bodyIndex = index;
      }
    }
    for (const [kind, mesh] of Object.entries(this.avatarBatches)) {
      mesh.count = counts[kind];
      mesh.instanceMatrix.needsUpdate = true;
      if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    }
  }

  makeNode(node) {
    const group = new THREE.Group();
    group.position.set(node.x, 0, node.y);
    group.name = node.id;
    const material = new THREE.MeshPhongMaterial({
      color: 0xa5f0c4,
      emissive: 0x397c60,
      emissiveIntensity: 0.7,
      shininess: 60,
      flatShading: true,
    });
    const crystal = this.addMesh(new THREE.OctahedronGeometry(15, 0), material, 0, 27, 0, group);
    crystal.scale.y = 1.45;
    for (const [x, z, s] of [
      [-14, 4, 0.6],
      [12, 7, 0.55],
    ]) {
      const shard = this.addMesh(crystal.geometry, material, x, 14, z, group);
      shard.scale.set(s, s * 1.5, s);
    }
    const ring = new THREE.Mesh(
      new THREE.RingGeometry(23, 26, 28),
      new THREE.MeshBasicMaterial({
        color: 0xbdfbd6,
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.45,
      }),
    );
    ring.rotation.x = -Math.PI / 2;
    ring.position.y = 0.6;
    group.add(ring);
    this.scene.add(group);
    return { group, crystal, material, ring };
  }

  disposeProjection(group) {
    this.scene.remove(group);
    const geometries = new Set(),
      materials = new Set();
    group.traverse((object) => {
      if (object.isMesh) geometries.add(object.geometry);
      if (object.material) materials.add(object.material);
    });
    for (const geometry of geometries) geometry.dispose();
    for (const material of materials) {
      material.map?.dispose();
      material.dispose();
    }
  }

  // 展台是无碰撞的装饰投影，一次只呈现当前选中旅人的服务器确认布置。
  setCamp(display) {
    const key = display ? JSON.stringify(display) : "";
    if (this.campProjection?.key === key) return;
    if (this.campProjection) this.disposeProjection(this.campProjection.group);
    this.campProjection = null;
    if (!display) return;
    const group = new THREE.Group();
    group.name = "camp-projection:" + display.owner_id;
    for (const placed of display.slots || []) {
      const slot = this.layout.camp.slots.find((item) => item.id === placed.slot_id);
      if (!slot) continue;
      const item = new THREE.Group();
      item.position.set(slot.x, 0, slot.y);
      item.userData.decorationID = placed.decoration_id;
      const gold = color(0xe9ce8b),
        teal = color(0x87c9c3);
      if (placed.decoration_id === "bell-lantern") {
        this.addMesh(new THREE.CylinderGeometry(5, 9, 13, 8), gold, 0, 14, 0, item);
        this.addMesh(new THREE.TorusGeometry(5, 1, 5, 12), teal, 0, 26, 0, item);
        this.addMesh(
          new THREE.SphereGeometry(3, 8, 6),
          new THREE.MeshBasicMaterial({ color: 0xfff0b9 }),
          0,
          7,
          0,
          item,
        );
      } else if (placed.decoration_id === "lake-vase") {
        this.addMesh(new THREE.CylinderGeometry(6, 8, 13, 9), teal, 0, 9, 0, item);
        this.addMesh(new THREE.CylinderGeometry(1, 1, 15, 5), gold, 0, 23, 0, item);
        this.addMesh(new THREE.OctahedronGeometry(7), color(0xd6a8bd), 0, 32, 0, item);
      } else if (placed.decoration_id === "wind-chime") {
        this.addMesh(new THREE.BoxGeometry(24, 3, 4), gold, 0, 34, 0, item);
        for (let i = -1; i <= 1; i++) {
          this.addMesh(new THREE.CylinderGeometry(0.7, 0.7, 12, 4), gold, i * 8, 27, 0, item);
          this.addMesh(new THREE.OctahedronGeometry(5), teal, i * 8, 16 - Math.abs(i) * 3, 0, item);
        }
      } else {
        gold.dispose();
        teal.dispose();
        continue;
      }
      const base = new THREE.Mesh(
        new THREE.RingGeometry(11, 13, 20),
        new THREE.MeshBasicMaterial({
          color: 0xb9edcc,
          side: THREE.DoubleSide,
          transparent: true,
          opacity: 0.7,
        }),
      );
      base.rotation.x = -Math.PI / 2;
      base.position.y = 0.5;
      item.add(base);
      item.traverse((object) => {
        if (object.isMesh) {
          object.castShadow = false;
          object.receiveShadow = false;
        }
      });
      group.add(item);
    }
    if (group.children.length) {
      const label = this.label(display.name + " · 微光展台", "#d6ffdc");
      label.position.set(this.layout.camp.x, 54, this.layout.camp.y);
      group.add(label);
    }
    this.scene.add(group);
    this.campProjection = { key, owner: display.owner_id, group };
    this.activityUntil = performance.now() + 750;
  }

  // 线索是无碰撞的私人提示；切换账号/步骤时及时释放本提示的GPU资源。
  setTrail(trail) {
    if (this.clue?.key === (trail ? trail.id + ":" + trail.step : "")) return;
    if (this.clue) {
      this.disposeProjection(this.clue.group);
      this.clue = null;
    }
    if (!trail) return;
    const group = new THREE.Group();
    group.name = "private-clue:" + trail.id;
    group.position.set(trail.x, trail.z, trail.y);
    const marker = this.addMesh(
      new THREE.OctahedronGeometry(9),
      new THREE.MeshBasicMaterial({ color: 0xffdc93 }),
      0,
      17,
      0,
      group,
    );
    marker.castShadow = false;
    const label = this.label("叶纹线索 · F 调查", "#ffe4a6");
    label.position.y = 42;
    group.add(label);
    group.visible = false;
    this.scene.add(group);
    this.clue = { group, marker, key: trail.id + ":" + trail.step };
    this.activityUntil = performance.now() + 750;
  }

  // 只比较角色空间状态，不因每次10Hz轮询的新对象而把静止场景误判成活动。
  frameDelay(now, state) {
    const players = state?.players || [];
    if (
      players.length !== this.players.size ||
      players.some((player) => {
        const entry = this.players.get(player.id);
        return (
          !entry ||
          Math.hypot(
            player.x - entry.group.position.x,
            player.y - entry.group.position.z,
            player.z - entry.group.position.y,
          ) > 0.1
        );
      })
    )
      this.activityUntil = now + 750;
    return now < this.activityUntil ? 1000 / 30 : 1000 / 12;
  }

  render(now, state, selfID) {
    const dt = this.lastTime ? Math.min(0.1, (now - this.lastTime) / 1000) : 0.016;
    this.lastTime = now;
    const smoothing = 1 - Math.exp(-dt * 15);
    const alive = new Set();
    for (const player of state?.players || []) {
      alive.add(player.id);
      let entry = this.players.get(player.id);
      if (!entry) {
        entry = this.makePlayer(player, player.id === selfID);
        this.players.set(player.id, entry);
        this.shadowDirty = true;
      }
      entry.target.set(player.x, player.z, player.y);
      if (Math.abs(entry.target.y - entry.group.position.y) > 0.05) this.shadowDirty = true;
      const dx = entry.target.x - entry.group.position.x,
        dz = entry.target.z - entry.group.position.z;
      if (Math.hypot(dx, dz) > 0.2) {
        this.shadowDirty = true;
        entry.group.rotation.y = Math.atan2(dx, dz);
        entry.legs[0].rotation.x = Math.sin(now / 95) * 0.5;
        entry.legs[1].rotation.x = -entry.legs[0].rotation.x;
      } else for (const leg of entry.legs) leg.rotation.x = 0;
      entry.group.position.lerp(entry.target, smoothing);
      entry.label.position.copy(entry.group.position).y += 82;
    }
    for (const [id, entry] of this.players)
      if (!alive.has(id)) {
        this.scene.remove(entry.group, entry.label);
        entry.label.material.map.dispose();
        entry.label.material.dispose();
        this.players.delete(id);
        this.shadowDirty = true;
      }
    for (const node of state?.nodes || []) {
      let entry = this.nodes.get(node.id);
      if (!entry) {
        entry = this.makeNode(node);
        this.nodes.set(node.id, entry);
        this.shadowDirty = true;
      }
      const ready = node.ready_at_ms <= (state?.time_ms || 0);
      entry.material.color.setHex(ready ? 0xa5f0c4 : 0x3b5851);
      entry.material.emissiveIntensity = ready ? 0.7 : 0;
      entry.ring.material.opacity = ready ? 0.45 : 0.12;
      entry.crystal.rotation.y = now / 1600;
      // 冷却只改变颜色/发光，不把仍有实体的晶簇缩小成看不见的空气墙。
      entry.crystal.position.y = 27 + (ready ? Math.sin(now / 650 + node.x) * 2 : 0);
      entry.crystal.scale.set(1, 1.45, 1);
    }
    this.drawAvatars();
    const me = this.players.get(selfID);
    if (this.clue) {
      this.clue.group.visible =
        !!me && me.group.position.distanceTo(this.clue.group.position) <= 180;
      this.clue.marker.rotation.y = now / 1400;
    }
    const projections =
      this.campProjection?.group.children.filter((item) => item.userData.decorationID) || [];
    this.setCanvasData("campOwner", this.campProjection?.owner || "");
    this.setCanvasData("campCount", projections.length);
    this.setCanvasData(
      "campKinds",
      projections.map((item) => item.userData.decorationID).join(","),
    );
    this.setCanvasData("clueVisible", !!this.clue?.group.visible);
    this.setCanvasData("clueId", this.clue?.key || "");
    if (me) {
      const follow = me.group.position.clone();
      follow.y += 16;
      this.target.lerp(follow, 1 - Math.exp(-dt * 8));
    }
    const desired = new THREE.Vector3(
      this.target.x + Math.sin(this.yaw) * this.distance,
      this.target.y + Math.tan(this.pitch) * this.distance,
      this.target.z + Math.cos(this.yaw) * this.distance,
    );
    this.camera.position.lerp(desired, 1 - Math.exp(-dt * 12));
    this.camera.lookAt(this.target);
    this.flame.scale.set(1 + Math.sin(now / 140) * 0.08, 1 + Math.cos(now / 120) * 0.12, 1);
    this.fire.intensity = 5000 + Math.sin(now / 100) * 600;
    if (this.shadowDirty && now - this.lastShadow >= 100) {
      this.renderer.shadowMap.needsUpdate = true;
      this.shadowDirty = false;
      this.lastShadow = now;
    }
    this.renderer.render(this.scene, this.camera);
    this.canvas.dataset.renderCount = String(++this.renderedFrames);
    if (me) this.avatarBatches.body.getMatrixAt(me.bodyIndex, this.avatarProbe);
    this.setCanvasData("selfHeight", me ? (this.avatarProbe.elements[13] - 25).toFixed(2) : "");
    this.setCanvasData("avatarInstances", this.avatarBatches.body.count);
    this.setCanvasData(
      "avatarBatches",
      Object.values(this.avatarBatches).filter((mesh) => mesh.count > 0).length,
    );
    this.setCanvasData(
      "npcCount",
      this.npcs.filter((entry) => entry.group.parent === this.scene).length,
    );
    this.setCanvasData("cameraYaw", this.yaw.toFixed(4));
    this.setCanvasData("cameraDistance", Math.round(this.distance));
    return {
      heading: Math.round(((((this.yaw * 180) / Math.PI) % 360) + 360) % 360),
      distance: Math.round(this.distance),
    };
  }
}
