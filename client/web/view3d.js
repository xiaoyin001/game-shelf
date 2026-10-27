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
    this.gl = gl;
    this.frameFences = [];
    this.frameQueuePeak = 0;
    window.addEventListener("pagehide", () => this.releaseFrames());
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 1.5));
    this.renderer.shadowMap.enabled = true;
    this.renderer.shadowMap.type = THREE.PCFShadowMap;
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
    this.makeCrystals();
    this.makeAvatarBatches();
    this.makePairing();
    this.npcs = layout.npcs.map((npc) => {
      const entry = this.makePlayer(
        { ...npc, z: 0, name: npc.role + " · " + npc.name },
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
      this.releaseFrames();
      onFailure("3D 上下文已丢失，请刷新页面；不会使用 2D 替代。");
    });
  }

  // 非阻塞检查GPU已完成的帧，保留原绘制目标但不继续堆积未完成命令。
  frameReady() {
    const gl = this.gl;
    if (gl.isContextLost()) return false;
    while (this.frameFences.length) {
      const status = gl.clientWaitSync(this.frameFences[0], 0, 0);
      if (status === gl.TIMEOUT_EXPIRED) break;
      if (status === gl.WAIT_FAILED) throw new Error("GPU帧同步失败");
      gl.deleteSync(this.frameFences.shift());
    }
    this.setCanvasData("gpuInFlight", this.frameFences.length);
    return this.frameFences.length < 2;
  }
  releaseFrames() {
    for (const fence of this.frameFences) this.gl.deleteSync(fence);
    this.frameFences.length = 0;
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
      mesh.setColorAt(0, new THREE.Color(0xffffff));
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

  // 背景晶簇不再冷却，以相同立体几何和材质合批；实体仍来自唯一场景。
  makeCrystals() {
    const nodes = this.layout.nodes;
    this.crystals = new THREE.InstancedMesh(
      new THREE.OctahedronGeometry(15, 0),
      new THREE.MeshPhongMaterial({
        color: 0xa5f0c4,
        emissive: 0x397c60,
        emissiveIntensity: 0.7,
        shininess: 60,
        flatShading: true,
      }),
      nodes.length * 3,
    );
    this.crystals.castShadow = true;
    this.crystals.receiveShadow = true;
    this.crystals.frustumCulled = false;
    const rings = new THREE.InstancedMesh(
      new THREE.RingGeometry(23, 26, 28),
      new THREE.MeshBasicMaterial({
        color: 0xbdfbd6,
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.45,
      }),
      nodes.length,
    );
    const d = new THREE.Object3D();
    d.rotation.x = -Math.PI / 2;
    nodes.forEach((n, i) => {
      d.position.set(n.x, 0.6, n.y);
      d.updateMatrix();
      rings.setMatrixAt(i, d.matrix);
    });
    this.scene.add(this.crystals, rings);
    this.crystalDummy = new THREE.Object3D();
    this.drawCrystals(0);
    this.setCanvasData("crystalInstances", this.crystals.count);
    this.setCanvasData("crystalBatches", 2);
  }

  drawCrystals(now) {
    const d = this.crystalDummy;
    this.layout.nodes.forEach((n, index) => {
      for (const [offset, x, y, z, s] of [
        [0, 0, 27 + Math.sin(now / 650 + n.x) * 2, 0, 1],
        [1, -14, 14, 4, 0.6],
        [2, 12, 14, 7, 0.55],
      ]) {
        d.position.set(n.x + x, y, n.y + z);
        d.rotation.set(0, offset === 0 ? now / 1600 : 0, 0);
        d.scale.set(s, s * (offset === 0 ? 1.45 : 1.5), s);
        d.updateMatrix();
        this.crystals.setMatrixAt(index * 3 + offset, d.matrix);
      }
    });
    this.crystals.instanceMatrix.needsUpdate = true;
  }

  // 异步预热不修改规则；完成前不允许表单开始进入，失败仍显式阻止游戏。
  async prepare() {
    this.drawAvatars();
    this.camera.lookAt(this.target);
    await this.renderer.compileAsync(this.scene, this.camera);
  }

  // 六类真实立体几何分批绘制，背包/台面/地面共享同一权威物品。
  makePairing() {
    this.pairBatches = new Map();
    const definitions = {
      mushroom: [new THREE.SphereGeometry(10, 9, 6, 0, Math.PI * 2, 0, Math.PI / 2), 0xe47568],
      pinecone: [new THREE.DodecahedronGeometry(8), 0xb18454],
      crystal: [new THREE.OctahedronGeometry(10), 0x81dce9],
      berry: [new THREE.SphereGeometry(9, 8, 6), 0xbf8de0],
      leaf: [new THREE.OctahedronGeometry(10), 0xa6dc75],
      shell: [new THREE.ConeGeometry(11, 15, 7), 0xe5c36c],
    };
    for (const [kind, [geometry, tint]] of Object.entries(definitions)) {
      const mesh = new THREE.InstancedMesh(geometry, color(tint), 16);
      mesh.count = 0;
      mesh.setColorAt(0, new THREE.Color(0xffffff));
      mesh.frustumCulled = false;
      mesh.castShadow = true;
      mesh.receiveShadow = true;
      this.pairBatches.set(kind, mesh);
      this.scene.add(mesh);
    }
    const a = this.layout.workshop;
    this.addMesh(new THREE.BoxGeometry(88, 6, 38), color(0xbba06c), a.x, 12, a.y);
    const ring = this.addMesh(
      new THREE.RingGeometry(a.range - 2, a.range, 48),
      new THREE.MeshBasicMaterial({
        color: 0xe9ce8b,
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.4,
      }),
      a.x,
      0.4,
      a.y,
    );
    ring.rotation.x = -Math.PI / 2;
    ring.castShadow = false;
    const label = this.label("配对台 · F 投送", "#ffe4a6");
    label.position.set(a.x, 56, a.y);
    this.scene.add(label);
    const spots = new THREE.InstancedMesh(
      new THREE.RingGeometry(15, 17, 20),
      new THREE.MeshBasicMaterial({
        color: 0xb9dfc3,
        side: THREE.DoubleSide,
        transparent: true,
        opacity: 0.5,
      }),
      a.spots.length,
    );
    const dummy = new THREE.Object3D();
    dummy.rotation.x = -Math.PI / 2;
    a.spots.forEach((p, i) => {
      dummy.position.set(p.x, 0.3, p.y);
      dummy.updateMatrix();
      spots.setMatrixAt(i, dummy.matrix);
    });
    this.scene.add(spots);
    this.pairFlash = this.addMesh(
      new THREE.TorusGeometry(28, 2, 5, 30),
      new THREE.MeshBasicMaterial({ color: 0xffe2a2, transparent: true }),
      a.x,
      24,
      a.y,
    );
    this.pairFlash.rotation.x = -Math.PI / 2;
    this.pairFlash.castShadow = false;
    this.pairFlash.visible = false;
    this.itemDummy = new THREE.Object3D();
    this.itemTint = new THREE.Color();
    this.pairGhosts = [];
  }

  setChallenge(round) {
    if (round && this.challenge?.id === round.id && round.pairs > this.challenge.pairs) {
      const live = new Set(round.items.map((i) => i.id));
      for (const item of this.challenge.items.filter((i) => !live.has(i.id)).slice(-6)) {
        const source = this.pairBatches.get(item.kind);
        if (!source) continue;
        const mesh = new THREE.Mesh(
          source.geometry,
          new THREE.MeshBasicMaterial({ color: source.material.color, transparent: true }),
        );
        const start = new THREE.Vector3();
        if (item.place === "hand" && this.players.has(item.owner_id)) {
          const owner = this.players.get(item.owner_id).group;
          owner.updateMatrixWorld(true);
          start.set((item.slot - 1) * 20, 70, -8).applyMatrix4(owner.matrixWorld);
        } else start.set(this.layout.workshop.x - 30 + item.slot * 15, 23, this.layout.workshop.y);
        this.scene.add(mesh);
        this.pairGhosts.push({ mesh, start, at: performance.now() });
      }
    }
    while (this.pairGhosts.length > 8 || (!round && this.pairGhosts.length)) {
      const { mesh } = this.pairGhosts.shift();
      this.scene.remove(mesh);
      mesh.material.dispose();
    }
    const stamp = JSON.stringify(round?.items || []);
    if (this.pairStamp !== stamp) {
      this.pairStamp = stamp;
      this.shadowDirty = true;
      this.activityUntil = performance.now() + 750;
    }
    this.challenge = round;
  }

  pairEffect(multiplier) {
    this.flashUntil = performance.now() + 650;
    this.flashMultiplier = multiplier;
    this.activityUntil = this.flashUntil;
    this.setCanvasData("feedbackPairs", this.challenge?.pairs || 0);
  }

  drawPairing(now) {
    const counts = new Map([...this.pairBatches.keys()].map((k) => [k, 0]));
    const a = this.layout.workshop,
      dummy = this.itemDummy;
    let hand = 0,
      tray = 0,
      ground = 0;
    for (const i of this.challenge?.items || []) {
      const mesh = this.pairBatches.get(i.kind);
      if (!mesh) continue;
      dummy.rotation.set(0, now / 3000, 0);
      dummy.scale.setScalar(1);
      if (i.kind === "pinecone") dummy.scale.set(0.85, 1.35, 0.85);
      if (i.kind === "leaf") {
        dummy.scale.set(0.7, 1.25, 0.28);
        dummy.rotation.z = 0.65;
      }
      if (i.place === "hand") {
        const owner = this.players.get(i.owner_id);
        if (!owner) continue;
        owner.group.updateMatrixWorld(true);
        dummy.position.set((i.slot - 1) * 20, 70, -8).applyMatrix4(owner.group.matrixWorld);
        hand++;
      } else if (i.place === "tray") {
        dummy.position.set(a.x - 30 + i.slot * 15, 23, a.y);
        dummy.scale.multiplyScalar(0.65);
        tray++;
      } else {
        dummy.position.set(i.x, i.z + 9, i.y);
        ground++;
      }
      dummy.updateMatrix();
      const index = counts.get(i.kind);
      mesh.setMatrixAt(index, dummy.matrix);
      this.itemTint.setHex(i.blocked ? 0x666666 : 0xffffff);
      mesh.setColorAt(index, this.itemTint);
      counts.set(i.kind, index + 1);
    }
    for (const [kind, mesh] of this.pairBatches) {
      mesh.count = counts.get(kind);
      mesh.instanceMatrix.needsUpdate = true;
      if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
    }
    this.pairGhosts = this.pairGhosts.filter((ghost) => {
      const p = Math.max(0, (now - ghost.at) / 450);
      if (p >= 1) {
        this.scene.remove(ghost.mesh);
        ghost.mesh.material.dispose();
        return false;
      }
      ghost.mesh.position.lerpVectors(ghost.start, new THREE.Vector3(a.x, 23, a.y), p * p);
      ghost.mesh.scale.setScalar(Math.max(0.01, 1 - p * 0.95));
      ghost.mesh.rotation.y = p * 6;
      ghost.mesh.material.opacity = 1 - p * 0.5;
      return true;
    });
    const progress = 1 - ((this.flashUntil || 0) - now) / 650;
    this.pairFlash.visible = progress >= 0 && progress < 1;
    if (this.pairFlash.visible) {
      this.pairFlash.scale.setScalar(0.5 + progress * 2.2);
      this.pairFlash.material.opacity = 1 - progress;
    }
    this.setCanvasData("groundItems", ground);
    this.setCanvasData("heldItems", hand);
    this.setCanvasData("trayItems", tray);
    this.setCanvasData(
      "itemBatches",
      [...this.pairBatches.values()].filter((m) => m.count > 0).length,
    );
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
    this.drawCrystals(now);
    this.drawAvatars();
    const me = this.players.get(selfID);
    this.drawPairing(now);
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
    const fence = this.gl.fenceSync(this.gl.SYNC_GPU_COMMANDS_COMPLETE, 0);
    if (!fence) throw new Error("无法追踪GPU帧完成");
    this.frameFences.push(fence);
    this.gl.flush();
    this.frameQueuePeak = Math.max(this.frameQueuePeak, this.frameFences.length);
    this.setCanvasData("gpuInFlight", this.frameFences.length);
    this.setCanvasData("gpuQueuePeak", this.frameQueuePeak);
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
