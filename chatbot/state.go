package chatbot

type State interface {
	getData() map[string]interface{}
	setData(data map[string]interface{})
	updateData(data map[string]interface{})
	getScene() Scene
	setScene(scene Scene)
}

type MapState struct {
	data  map[string]interface{}
	scene Scene
}

func NewMapState(data map[string]interface{}, scene Scene) *MapState {
	newData := make(map[string]interface{})
	for key, value := range data {
		newData[key] = value
	}
	return &MapState{
		data:  newData,
		scene: scene,
	}
}

func (s *MapState) getData() map[string]interface{} {
	return s.data
}

func (s *MapState) setData(data map[string]interface{}) {
	s.data = data
}

func (s *MapState) updateData(data map[string]interface{}) {
	for key, value := range data {
		s.data[key] = value
	}
}

func (s *MapState) getScene() Scene {
	return s.scene
}

func (s *MapState) setScene(scene Scene) {
	s.scene = scene
}
