package ttdirender

import (
	"fmt"
	"log"
	"strconv"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	g3ndpalette "github.com/mrjrieke/nute/g3nd/palette"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
)

type ClickedG3nDetailElement struct {
	clickedElement *g3nmash.G3nDetailedElement
	center         *math32.Vector3
}

var maxTime int

type ElementRenderer struct {
	g3nrender.GenericRenderer
	iOffset         int
	counter         float64
	totalElements   int
	LocationCache   map[int64]*math32.Vector3
	clickedElements []*ClickedG3nDetailElement
}

// Returns true if length of er.clickedElements stack is 0 and false otherwise
func (er *ElementRenderer) isEmpty() bool {
	return len(er.clickedElements) == 0
}

// Returns size of er.clickedElements stack
func (er *ElementRenderer) length() int {
	return len(er.clickedElements)
}

// Adds given element and location to the er.clickedElements stack
func (er *ElementRenderer) push(g3nDetailedElement *g3nmash.G3nDetailedElement, centerLocation *math32.Vector3) {
	clickedElements := ClickedG3nDetailElement{
		clickedElement: g3nDetailedElement,
		center:         centerLocation,
	}
	er.clickedElements = append(er.clickedElements, &clickedElements)
}

// Removes and returns top element in er.clickedElements stack
func (er *ElementRenderer) pop() *ClickedG3nDetailElement {
	size := len(er.clickedElements)
	element := er.clickedElements[size-1]
	er.clickedElements = er.clickedElements[:size-1]
	return element
}

// Returns top element in er.clickedElements stack
func (er *ElementRenderer) top() *ClickedG3nDetailElement {
	return er.clickedElements[er.length()-1]
}

// Returns and attaches a mesh to provided g3n element at given vector position
func (er *ElementRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	sphereGeom := geometry.NewSphere(.1, 100, 100)
	color := g3ndpalette.DARK_BLUE
	if g3n.GetDetailedElement().Alias == "Argosy" {
		if g3n.GetDetailedElement().Data != "" {
			fmt.Println(g3n.GetDetailedElement().Data)
			maxTime, _ = strconv.Atoi(g3n.GetDetailedElement().Data)
		}
		color.Set(0, 0.349, 0.643)
	} else if g3n.GetDetailedElement().Alias == "DataFlowGroup" {
		color.Set(1.0, 0.224, 0.0)
	} else if g3n.GetDetailedElement().Alias == "DataFlow" {
		color = math32.NewColor("olive")
	}
	mat := material.NewStandard(color)
	sphereMesh := graphic.NewMesh(sphereGeom, mat)
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	return sphereMesh
}

func (er *ElementRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

// Returns the element and location of the given element
func (er *ElementRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	er.totalElements = totalElements
	if er.iOffset >= 2 {
		id := g3n.GetDisplayId()
		return g3n, er.LocationCache[id]
	} else {
		if er.iOffset == 0 {
			er.iOffset = 1
			er.counter = 0.0
			er.LocationCache = make(map[int64]*math32.Vector3)
			er.LocationCache[g3n.GetDetailedElement().Id] = math32.NewVector3(0, 0, 0)
			return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
		} else {
			er.counter = er.counter - 0.1
			complex := binetFormula(er.counter)
			er.LocationCache[g3n.GetDetailedElement().Id] = math32.NewVector3(float32(-real(complex)), float32(imag(complex)), float32(-er.counter))
			return g3n, math32.NewVector3(float32(-real(complex)), float32(imag(complex)), float32(-er.counter))
		}
	}
}

// Calls LayoutBase to render elements in a particular order and location
func (er *ElementRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	er.LayoutBase(worldApp, er, g3nRenderableElements)
}

// Removes elements that are no longer clicked or needing to be shown
func (er *ElementRenderer) deselectElements(worldApp *g3nworld.WorldApp, element *g3nmash.G3nDetailedElement) *g3nmash.G3nDetailedElement {
	for _, childID := range element.GetChildElementIds() {
		if !er.isChildElement(worldApp, element) && element != worldApp.ClickedElements[len(worldApp.ClickedElements)-1] {
			worldApp.ConcreteElements[childID].ApplyState(mashupsdk.Hidden, true)
			er.RemoveAll(worldApp, childID)
		}
	}
	if len(element.GetParentElementIds()) == 0 || (len(worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetParentElementIds()) != 0 && element.GetParentElementIds()[0] == worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetParentElementIds()[0]) {
		return element
	} else {
		return er.deselectElements(worldApp, worldApp.ConcreteElements[element.GetParentElementIds()[0]])
	}
}

// Populates location cache by parent id of given ids
func (er *ElementRenderer) calcLocnStarter(worldApp *g3nworld.WorldApp, ids []int64, counter float32) {
	for _, id := range ids {
		element := worldApp.ConcreteElements[id]
		for _, parent := range element.GetParentElementIds() {
			if parent > 0 && er.LocationCache[parent] != nil {
				er.LocationCache[id] = er.calculateLocation(er.LocationCache[parent], counter)
				counter = counter - 0.1
			}
		}
	}
}

// Returns location of spiral centered at the provided center
func (er *ElementRenderer) calculateLocation(center *math32.Vector3, counter float32) *math32.Vector3 {
	complex := binetFormula(float64(counter))
	return math32.NewVector3(float32(-real(complex))+center.X, float32(imag(complex))+center.Y, float32(-counter)+center.Z)
}

// Initializes location cache
func (er *ElementRenderer) initLocnCache(worldApp *g3nworld.WorldApp, element *g3nmash.G3nDetailedElement) {
	if len(element.GetChildElementIds()) != 0 {
		if element.GetDetailedElement().Genre != "Solid" {
			er.calcLocnStarter(worldApp, element.GetChildElementIds(), -0.1)
		}

		for _, childID := range element.GetChildElementIds() {
			if worldApp.ConcreteElements[childID].GetDetailedElement().Genre != "Solid" {
				er.initLocnCache(worldApp, worldApp.ConcreteElements[childID])
			}
		}
	}
}

// Properly sets the elements before rendering new clicked elements
func (er *ElementRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	//Initialize location cache
	if er.iOffset != 2 {
		copyCache := make(map[int64]*math32.Vector3)
		for k, v := range er.LocationCache {
			copyCache[k] = v
		}
		for key, _ := range copyCache {
			element := worldApp.ConcreteElements[key]
			if element.GetDetailedElement().Genre != "Solid" {
				er.initLocnCache(worldApp, element)
			}
		}
		er.iOffset = 2
	}
	if !er.isEmpty() {
		prevElement := er.top()
		if !er.isChildElement(worldApp, prevElement.clickedElement) && prevElement != nil && prevElement.clickedElement.GetDetailedElement().Genre != "Solid" && worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetDetailedElement().Genre != "Space" {
			er.pop()
			for _, childID := range prevElement.clickedElement.GetChildElementIds() {
				if !er.isChildElement(worldApp, prevElement.clickedElement) {
					worldApp.ConcreteElements[childID].ApplyState(mashupsdk.Hidden, true)
					er.RemoveAll(worldApp, childID)
				}
			}
			er.deselectElements(worldApp, prevElement.clickedElement)
		}
	}
	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	if clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "Space" {
		name := clickedElement.GetDisplayName()
		mesh := clickedElement.GetNamedMesh(name)
		pos := mesh.Position()
		center := pos
		er.push(clickedElement, &center)
		for _, childID := range clickedElement.GetChildElementIds() {
			childElement := worldApp.ConcreteElements[childID]
			if childElement.GetDetailedElement().Genre != "Solid" {
				childElement.ApplyState(mashupsdk.Hidden, false)
				childElement.ApplyState(mashupsdk.Clicked, true)
			}
		}
	}
	return true
}

// Checks if the currently clicked element is a child of the provided element
func (er *ElementRenderer) isChildElement(worldApp *g3nworld.WorldApp, prevElement *g3nmash.G3nDetailedElement) bool {
	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	for _, childID := range prevElement.GetChildElementIds() {
		if clickedElement == worldApp.ConcreteElements[childID] {
			return true
		}
	}
	return false
}

// Renders provided element if it was clicked
// Returns true if given element is the last clicked element and false otherwise
func (er *ElementRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3n *g3nmash.G3nDetailedElement) bool {
	if g3n == worldApp.ClickedElements[len(worldApp.ClickedElements)-1] && g3n.GetNamedMesh(g3n.GetDisplayName()) != nil {
		g3n.SetColor(math32.NewColor("darkred"), 1.0)

		for _, childId := range g3n.GetChildElementIds() {
			element := worldApp.ConcreteElements[childId]
			if element.GetDetailedElement().Genre != "Solid" && element.GetDetailedElement().Alias != "DataFlowStatistic" {
				if element.GetNamedMesh(element.GetDisplayName()) == nil {
					_, nextPos := er.NextCoordinate(element, er.totalElements)
					solidMesh := er.NewSolidAtPosition(element, nextPos)
					if solidMesh != nil {
						log.Printf("Adding %s\n", solidMesh.GetNode().LoaderID())
						worldApp.UpsertToScene(solidMesh)
						element.SetNamedMesh(element.GetDisplayName(), solidMesh)
					}
				} else {
					worldApp.UpsertToScene(element.GetNamedMesh(element.GetDisplayName()))
				}
			}
		}
		return true
	} else {
		color := g3ndpalette.DARK_BLUE
		if g3n.GetDetailedElement().Alias == "Argosy" {
			color.Set(0, 0.349, 0.643)
		} else if g3n.GetDetailedElement().Alias == "DataFlowGroup" {
			color.Set(1.0, 0.224, 0.0)
		} else if g3n.GetDetailedElement().Alias == "DataFlow" {
			color = math32.NewColor("olive")
		}

		clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
		for _, childID := range clickedElement.GetChildElementIds() {
			if g3n == worldApp.ConcreteElements[childID] {
				g3n.SetColor(color, 1.0)
				return true
			} else {
				g3n.SetColor(color, 0.15)
			}
		}
	}
	return true
}

//Removes all children nodes of provided id
func (er *ElementRenderer) RemoveAll(worldApp *g3nworld.WorldApp, childId int64) {
	if child, childOk := worldApp.ConcreteElements[childId]; childOk {
		if !child.IsAbstract() && child.GetDetailedElement().Genre != "Solid" {
			if childMesh := child.GetNamedMesh(child.GetDisplayName()); childMesh != nil {
				log.Printf("Child Item removed %s: %v", child.GetDisplayName(), worldApp.RemoveFromScene(childMesh))
			}
		}

		if len(child.GetChildElementIds()) > 0 {
			for _, cId := range child.GetChildElementIds() {
				er.RemoveAll(worldApp, cId)
			}
		}
	}
}

// Adds elements to scene
func (er *ElementRenderer) LayoutBase(worldApp *g3nworld.WorldApp,
	g3Renderer *ElementRenderer,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	var nextPos *math32.Vector3
	var prevSolidPos *math32.Vector3

	totalElements := len(g3nRenderableElements)

	if totalElements > 0 {
		if g3nRenderableElements[0].GetDetailedElement().Colabrenderer != "" {
			log.Printf("Collab examine: %v\n", g3nRenderableElements[0])
			log.Printf("Renderer name: %s\n", g3nRenderableElements[0].GetDetailedElement().GetRenderer())
			protoRenderer := g3Renderer.GetRenderer(g3nRenderableElements[0].GetDetailedElement().GetRenderer())
			log.Printf("Collaborating %v\n", protoRenderer)
			g3Renderer.Collaborate(worldApp, protoRenderer)
		}
	}

	for _, g3nRenderableElement := range g3nRenderableElements {
		concreteG3nRenderableElement := g3nRenderableElement
		if !g3nRenderableElement.IsStateSet(mashupsdk.Hidden) {
			prevSolidPos = nextPos
			_, nextPos = g3Renderer.NextCoordinate(concreteG3nRenderableElement, totalElements)
			solidMesh := g3Renderer.NewSolidAtPosition(concreteG3nRenderableElement, nextPos)
			if solidMesh != nil {
				log.Printf("Adding %s\n", solidMesh.GetNode().LoaderID())
				worldApp.UpsertToScene(solidMesh)
				concreteG3nRenderableElement.SetNamedMesh(concreteG3nRenderableElement.GetDisplayName(), solidMesh)
			}

			for _, relatedG3n := range worldApp.GetG3nDetailedChildElementsByGenre(concreteG3nRenderableElement, "Related") {
				relatedMesh := g3Renderer.NewRelatedMeshAtPosition(concreteG3nRenderableElement, nextPos, prevSolidPos)
				if relatedMesh != nil {
					worldApp.UpsertToScene(relatedMesh)
					concreteG3nRenderableElement.SetNamedMesh(relatedG3n.GetDisplayName(), relatedMesh)
				}
			}

			for _, innerG3n := range worldApp.GetG3nDetailedChildElementsByGenre(concreteG3nRenderableElement, "Space") {
				negativeMesh := g3Renderer.NewInternalMeshAtPosition(innerG3n, nextPos)
				if negativeMesh != nil {
					worldApp.UpsertToScene(negativeMesh)
					innerG3n.SetNamedMesh(innerG3n.GetDisplayName(), negativeMesh)
				}
			}
		}

	}
}

func (er *ElementRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer g3nrender.IG3nRenderer) {
	curveRenderer := collaboratingRenderer.(*CurveRenderer)
	curveRenderer.maxTime = maxTime
	curveRenderer.er = er
}