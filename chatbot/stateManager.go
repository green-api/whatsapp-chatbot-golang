package chatbot

type StateManager interface {
	Get(stateId string) State
	Create(stateId string) State
	Update(stateId string)
	Delete(stateId string)
	GetStateData(stateId string) map[string]interface{}
	SetStateData(stateId string, stateData map[string]interface{})
	UpdateStateData(stateId string, stateData map[string]interface{})
	DeleteStateData(stateId string)
	ActivateNextScene(stateId string, scene Scene)
	GetCurrentScene(stateId string) Scene
	GetStartScene() Scene
	SetStartScene(startScene Scene)
}

type MapStateManager struct {
	states     map[string]State
	InitData   map[string]interface{}
	StartScene Scene
}

func NewMapStateManager(initData map[string]interface{}) *MapStateManager {
	return &MapStateManager{
		states:     map[string]State{},
		InitData:   initData,
		StartScene: nil,
	}
}

func (sm *MapStateManager) GetStartScene() Scene {
	return sm.StartScene
}

func (sm *MapStateManager) SetStartScene(StartScene Scene) {
	sm.StartScene = StartScene
}

func (sm *MapStateManager) Get(stateId string) State {
	if state, exist := sm.states[stateId].(State); exist {
		return state
	}
	return nil
}

func (sm *MapStateManager) Create(stateId string) State {
	sm.states[stateId] = NewMapState(sm.InitData, sm.StartScene)
	return sm.Get(stateId)
}

func (sm *MapStateManager) Update(stateId string) {
}

func (sm *MapStateManager) Delete(stateId string) {
	delete(sm.states, stateId)
}

func (sm *MapStateManager) GetStateData(stateId string) map[string]interface{} {
	state := sm.Get(stateId)
	if state != nil {
		return state.getData()
	}
	return nil
}

func (sm *MapStateManager) SetStateData(stateId string, newStateData map[string]interface{}) {
	state := sm.Get(stateId)
	if state != nil {
		state.setData(newStateData)
	}
}

func (sm *MapStateManager) UpdateStateData(stateId string, newStateData map[string]interface{}) {
	state := sm.Get(stateId)
	if state != nil {
		state.updateData(newStateData)
	}
}

func (sm *MapStateManager) DeleteStateData(stateId string) {
	state := sm.Get(stateId)
	if state != nil {
		state.setData(sm.InitData)
	}
}

func (sm *MapStateManager) ActivateNextScene(stateId string, scene Scene) {
	state := sm.Get(stateId)
	if state != nil {
		state.setScene(scene)
	}
}

func (sm *MapStateManager) GetCurrentScene(stateId string) Scene {
	state := sm.Get(stateId)
	if state != nil {
		return state.getScene()
	}
	return nil
}
