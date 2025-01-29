package app

import (
	"fmt"
	"time"

	"github.com/g3n/engine/app"
	"github.com/g3n/engine/camera"
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/light"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/renderer"
	"github.com/g3n/engine/util"
	"github.com/g3n/engine/util/helper"
	"github.com/g3n/engine/window"
)

const (
	targetFPS = 60
)

type App struct {
	*app.Application
	scene      *core.Node
	vizScene   *core.Node
	frameRater *util.FrameRater

	// GUI
	mainPanel *gui.Panel
	// vizPanel  *gui.Panel
	labelFPS *gui.Label

	// Scene
	camera *camera.Camera
	orbit  *camera.OrbitControl
}

func Create() *App {
	a := new(App)
	a.Application = app.App()
	fmt.Println("Starting OF_IMU_LocationCore-Viz...")

	// Log OpenGL version
	glVersion := a.Gls().GetString(gls.VERSION)
	fmt.Printf("OpenGL ver: %s\n", glVersion)

	// Create scenes
	a.vizScene = core.NewNode()
	a.scene = core.NewNode()
	a.scene.Add(a.vizScene)

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

	// Create vizPanel

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

	// Add viz panel

	// Serial Monitor Footer
	footer := gui.NewPanel(float32(width), float32(height)*0.2)
	footer.SetBorders(1, 0, 0, 0)
	footer.SetPaddings(2, 2, 2, 2)
	footer.SetColor4(&math32.Color4{R: 0.25, G: 0.25, B: 0.25, A: 0.75})
	footer.SetLayoutParams(&gui.DockLayoutParams{Edge: gui.DockBottom})
	a.mainPanel.Add(footer)

	srm := gui.NewVScroller(float32(width), float32(height)*0.2)
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
	fps, _, ok := a.frameRater.FPS(time.Duration(targetFPS) * time.Millisecond)
	if !ok {
		return
	}
	a.labelFPS.SetText(fmt.Sprintf("FPS: %3.1f", fps))
}
