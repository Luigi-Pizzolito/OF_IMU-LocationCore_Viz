package app

import (
	"fmt"
	"time"

	"github.com/g3n/engine/app"
	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/light"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/renderer"
	"github.com/g3n/engine/util"
	"github.com/g3n/engine/util/helper"
	"github.com/g3n/engine/window"
)

const (
	targetFPS = 60
)

//todo: move all of the stack-initialsied members to class properties for global access

type App struct {
	*app.Application
	scene *core.Node

	// GUI
	mainPanel *gui.Panel

	// Scene
	camera *camera.Camera
	orbit  *camera.OrbitControl

	vdisk_g *geometry.Geometry
	vcube_g *geometry.Geometry
	mat1    *material.Standard
	mat2    *material.Standard
	vdisk   *graphic.Mesh
	vcube   *graphic.Mesh

	trail_m *material.Standard
	trail_s []*graphic.Sprite

	frameRater *util.FrameRater
	labelFPS   *gui.Label
	t          time.Duration
}

func Create() *App {
	a := new(App)
	a.Application = app.App()
	fmt.Println("Starting OF_IMU_LocationCore-Viz...")

	// Log OpenGL version
	glVersion := a.Gls().GetString(gls.VERSION)
	fmt.Printf("OpenGL ver: %s\n", glVersion)

	// Create scenes
	a.scene = core.NewNode()

	// Create camera & orbit control
	width, height := a.GetSize()
	aspect := float32(width) / float32(height)
	a.camera = camera.New(aspect)
	a.camera.SetPosition(0, 2, 3)
	a.camera.LookAt(&math32.Vector3{X: 0, Y: 0, Z: 0}, &math32.Vector3{X: 0, Y: 1, Z: 0})
	a.scene.Add(a.camera)
	a.orbit = camera.NewOrbitControl(a.camera)

	// Create frame rater
	a.frameRater = util.NewFrameRater(targetFPS)

	// Build user interface
	a.buildGUI()

	// window resize handler
	a.Subscribe(window.OnWindowSize, func(evname string, ev interface{}) {
		a.OnWindowResize()
	})
	a.OnWindowResize()

	// key events

	// Setup scene
	a.setupScene()

	return a
}

func (a *App) OnWindowResize() {
	width, height := a.GetFramebufferSize()
	a.Gls().Viewport(0, 0, int32(width), int32(height))
	a.camera.SetAspect(float32(width) / float32(height))

	a.mainPanel.SetSize(float32(width), float32(height))
}

func (a *App) setupScene() {
	// Set background color
	a.Gls().ClearColor(0.05, 0.05, 0.05, 1.0)

	// Ambient light
	ambientLight := light.NewAmbient(&math32.Color{R: 1.0, G: 1.0, B: 1.0}, 0.8)
	a.scene.Add(ambientLight)

	// Helper axis
	haxis := helper.NewAxes(1)
	a.scene.Add(haxis)

	// Helper grid
	hgrid := helper.NewGrid(20, 1, &math32.Color{R: 0.5, G: 0.5, B: 0.5})
	a.scene.Add(hgrid)

	// Create a disk geometry
	a.vdisk_g = geometry.NewDisk(1, 3)
	a.mat1 = material.NewStandard(&math32.Color{R: 1, G: 0, B: 1})
	a.mat1.SetWireframe(true)
	a.mat1.SetLineWidth(2)
	a.vdisk = graphic.NewMesh(a.vdisk_g, a.mat1)
	a.vdisk.SetRotation(-math32.Pi/2, 0, 0)
	a.scene.Add(a.vdisk)

	// Create a cube geometry
	a.vcube_g = geometry.NewCube(1)
	a.mat2 = material.NewStandard(&math32.Color{R: 1, G: 1, B: 0})
	a.mat2.SetWireframe(true)
	a.mat2.SetLineWidth(2)
	a.vcube = graphic.NewMesh(a.vcube_g, a.mat2)
	a.vcube.SetScale(0.5, 0.5, 0.5)
	a.vcube.SetPosition(0, 0, 0.25)
	a.vdisk.Add(a.vcube)

	// Create a trail sprites
	a.trail_m = material.NewStandard(&math32.Color{R: 0, G: 1, B: 1})
	a.trail_m.SetTransparent(true)
	a.trail_m.SetOpacity(0.5)
	// a.trail_m.SetEmissiveColor(&math32.Color{R: 0, G: 1, B: 1})
	a.trail_s = make([]*graphic.Sprite, 100)
	for i := 0; i < 100; i++ {
		a.trail_s[i] = graphic.NewSprite(0.2, 0.1, a.trail_m)
		a.trail_s[i].SetPosition(0, 0, 0)
		a.scene.Add(a.trail_s[i])
	}

}

func (a *App) buildGUI() {
	// Create dock layout
	dl := gui.NewDockLayout()
	width, height := a.GetSize()
	a.mainPanel = gui.NewPanel(float32(width), float32(height))
	a.mainPanel.SetRenderable(false)
	a.mainPanel.SetEnabled(false)
	a.mainPanel.SetLayout(dl)
	a.scene.Add(a.mainPanel)
	gui.Manager().Set(a.mainPanel)

	// Serial Monitor Footer
	footer := gui.NewPanel(float32(width), float32(height)*0.2)
	footer.SetBorders(1, 0, 0, 0)
	footer.SetPaddings(2, 2, 2, 2)
	footer.SetColor4(&math32.Color4{R: 0.25, G: 0.25, B: 0.25, A: 0.75})
	footer.SetLayoutParams(&gui.DockLayoutParams{Edge: gui.DockBottom})
	a.mainPanel.Add(footer)

	srm := gui.NewVScroller(float32(width), float32(height)*0.2)
	a.mainPanel.SubscribeID(gui.OnResize, a, func(evname string, ev interface{}) {
		width, height := a.GetSize()
		srm.SetSize(float32(width), float32(height)*0.2)
	})
	// srm.SetColor(&math32.Color{R: 1, G: 1, B: 1})
	for i := 1; i <= 100; i++ {
		srm.Add(gui.NewLabel(fmt.Sprintf("label%d", i)))
	}
	srm.ScrollDown()
	footer.Add(srm)

	// Graph & Table sidebar
	sidebar := gui.NewPanel(float32(width)*0.3, float32(height))
	sidebar.SetBorders(0, 0, 0, 1)
	sidebar.SetPaddings(2, 2, 2, 2)
	sidebar.SetColor4(&math32.Color4{R: 0.25, G: 0.25, B: 0.25, A: 0.75})
	sidebar.SetLayoutParams(&gui.DockLayoutParams{Edge: gui.DockRight})
	a.mainPanel.Add(sidebar)

	sidebar_v := gui.NewVBoxLayout()
	sidebar_v.SetSpacing(5)
	sidebar_v.SetAutoHeight(true)
	sidebar_v.SetAutoWidth(false)
	sidebar_v.SetAlignV(gui.AlignTop)
	sidebar.SetLayout(sidebar_v)

	// Trail slider
	trail_l := gui.NewLabel("Trail length: ")
	sidebar.Add(trail_l)
	trail_s := gui.NewHSlider(sidebar.Width(), trail_l.Height())
	trail_s.SetValue(1)
	trail_s.SetText(fmt.Sprintf("%d frames", int(trail_s.Value()*100)))
	trail_s.Subscribe(gui.OnChange, func(evname string, ev interface{}) {
		// process change
		trail_s.SetText(fmt.Sprintf("%d frames", int(trail_s.Value()*100)))
	})
	sidebar.Add(trail_s)

	// Graphs
	graphs_tb_l := gui.NewLabel("Sensor Data: ")
	sidebar.Add(graphs_tb_l)
	graphs_tb := gui.NewTabBar(sidebar.Width()-4, graphs_tb_l.Height()*10)
	graphs_tb.SetPaddings(0, 2, 0, 2)
	graphs_tb.SetMargins(0, 2, 0, 2)
	sidebar.Add(graphs_tb)
	a.mainPanel.SubscribeID(gui.OnResize, a, func(evname string, ev interface{}) {
		graphs_tb.SetSize(sidebar.Width()-4, graphs_tb_l.Height()*10)
	})

	// accel graph
	graphs_accel_tab := graphs_tb.AddTab("Linear Accel.")
	graphs_accel_tab.SetPinned(true)

	graph_imu_accel := gui.NewChart(sidebar.Width()-16, graphs_tb_l.Height()*10)
	graph_imu_accel.SetMargins(0, 2, 0, 2)
	graph_imu_accel.SetBorders(2, 2, 2, 2)
	graph_imu_accel.SetBordersColor(math32.NewColor("black"))
	graph_imu_accel.SetPaddings(0, 2, 0, 2)
	graph_imu_accel.SetColor(math32.NewColor("white"))
	graph_imu_accel.SetRangeY(-2, 2)
	graph_imu_accel.SetScaleY(10, &math32.Color{R: 0.8, G: 0.8, B: 0.8})
	graph_imu_accel.SetFontSizeX(12)
	graph_imu_accel.SetFormatY("%2.1f")
	graphs_accel_tab.SetContent(graph_imu_accel)

	// orientation graph
	graphs_orio_tab := graphs_tb.AddTab("Orientation")
	graphs_orio_tab.SetPinned(true)

	graph_imu_orio := gui.NewChart(sidebar.Width()-16, graphs_tb_l.Height()*10)
	graph_imu_orio.SetMargins(0, 2, 0, 2)
	graph_imu_orio.SetBorders(2, 2, 2, 2)
	graph_imu_orio.SetBordersColor(math32.NewColor("black"))
	graph_imu_orio.SetPaddings(0, 2, 0, 2)
	graph_imu_orio.SetColor(math32.NewColor("white"))
	graph_imu_orio.SetRangeY(-180, 180)
	graph_imu_orio.SetScaleY(10, &math32.Color{R: 0.8, G: 0.8, B: 0.8})
	graph_imu_orio.SetFontSizeX(12)
	graph_imu_orio.SetFormatY("%2.1f")
	graphs_orio_tab.SetContent(graph_imu_orio)

	// OF graph
	graphs_of_tab := graphs_tb.AddTab("Optical Flow")
	graphs_of_tab.SetPinned(true)

	// Kalman parameters viewer
	//todo: dont use tabs but show everything at once? nested panels?

	kalman_tb_l := gui.NewLabel("Kalman Parameters:")
	sidebar.Add(kalman_tb_l)
	kalman_tb := gui.NewTabBar(sidebar.Width()-4, kalman_tb_l.Height()*10)
	kalman_tb.SetPaddings(0, 2, 0, 2)
	kalman_tb.SetMargins(0, 2, 0, 2)
	sidebar.Add(kalman_tb)
	a.mainPanel.SubscribeID(gui.OnResize, a, func(evname string, ev interface{}) {
		kalman_tb.SetSize(sidebar.Width()-4, kalman_tb_l.Height()*10)
	})

	// State
	kalman_state_tab := kalman_tb.AddTab("State (x)")
	kalman_state_tab.SetPinned(true)

	kalman_pc_tab := kalman_tb.AddTab("Process Covariance (Q)")
	kalman_pc_tab.SetPinned(true)

	kalman_oc_tab := kalman_tb.AddTab("Observation Covariance (R)")
	kalman_oc_tab.SetPinned(true)

	//

	// FPS label
	a.labelFPS = gui.NewLabel("FPS: 000.0")
	a.labelFPS.SetColor(&math32.Color{R: 1, G: 1, B: 1})
	a.mainPanel.Add(a.labelFPS)

	// Return focus to viz scene when leaving GUI
	a.mainPanel.Subscribe(gui.OnCursorLeave, func(name string, ev interface{}) {
		gui.Manager().SetKeyFocus(nil)
	})
}

func (a *App) Run() {
	a.Application.Run(a.Update)
}

func (a *App) Update(rend *renderer.Renderer, deltaTime time.Duration) {
	// Start measuring this frame
	a.frameRater.Start()

	// Clear the color, depth and stencil buffers
	a.Gls().Clear(gls.DEPTH_BUFFER_BIT | gls.STENCIL_BUFFER_BIT | gls.COLOR_BUFFER_BIT)

	// Update Viz scene
	a.updateViz(deltaTime)

	// Render scene
	err := rend.Render(a.scene, a.camera)
	if err != nil {
		panic(err)
	}

	// Update GUI timers
	gui.Manager().TimerManager.ProcessTimers()

	// Control and update FPS
	a.frameRater.Wait()
	a.updateFPS()
}

func (a *App) updateFPS() {
	fps, ffps, ok := a.frameRater.FPS(time.Duration(targetFPS) * time.Millisecond)
	if !ok {
		return
	}
	a.labelFPS.SetText(fmt.Sprintf("Render FPS: %3.1f\nMax FPS: %4.f", fps, ffps))
}

func (a *App) updateViz(deltaTime time.Duration) {
	// Rotate the disk
	a.vdisk.RotateZ(0.01)
	// Move the disk along a circular path
	a.t += deltaTime
	timeElapsed := float64(a.t.Seconds())
	radius := 2.0
	speed := 2.0
	angle := speed * timeElapsed

	x := float32(radius) * math32.Cos(float32(angle))
	z := float32(radius) * math32.Sin(float32(angle))
	a.vdisk.SetPosition(
		x,
		0,
		z,
	)

	// Update the trail sprites
	for i := len(a.trail_s) - 1; i > 0; i-- {
		pos := a.trail_s[i-1].Position()
		a.trail_s[i].SetPositionVec(&pos)
		rot := a.trail_s[i-1].Rotation()
		a.trail_s[i].SetRotationVec(&rot)
	}
	a.trail_s[0].SetPosition(x, 0, z)
	a.trail_s[0].RotateZ(0.01)
}
