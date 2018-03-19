package timer

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/TIBCOSoftware/flogo-lib/core/action"
	"github.com/TIBCOSoftware/flogo-lib/core/trigger"
	"github.com/TIBCOSoftware/flogo-lib/logger"
	"github.com/carlescere/scheduler"
)

// log is the default package logger
var log = logger.GetLogger("trigger-jvanderl-timer")

//TimerTrigger is th main structure for this trigger
type TimerTrigger struct {
	metadata *trigger.Metadata
	//runner   action.Runner
	config *trigger.Config
	timers map[string]*scheduler.Job
}

//NewFactory create a new Trigger factory
func NewFactory(md *trigger.Metadata) trigger.Factory {
	return &TimerFactory{metadata: md}
}

// TimerFactory Timer Trigger factory
type TimerFactory struct {
	metadata *trigger.Metadata
}

//New Creates a new trigger instance for a given id
func (t *TimerFactory) New(config *trigger.Config) trigger.Trigger {
	return &TimerTrigger{metadata: t.metadata, config: config}
}

// Metadata implements trigger.Trigger.Metadata
func (t *TimerTrigger) Metadata() *trigger.Metadata {
	return t.metadata
}

// Init implements ext.Trigger.Init
func (t *TimerTrigger) Init(runner action.Runner) {
	log.Debug("Trigger Init called")
	//	t.runner = runner
	//	log.Infof("In init, id: '%s', Metadata: '%+v', Config: '%+v'", t.config.Id, t.metadata, t.config)
}

// Initialize implements ext.Trigger.Initialize
func (t *TimerTrigger) Initialize(ctx trigger.InitContext) error {
	log.Debug("Trigger Initialize called")
	t.timers = make(map[string]*scheduler.Job)
	//handlers := t.config.Handlers

	for _, handler := range ctx.GetHandlers() {
		repeating := handler.GetStringSetting("repeating")
		//repeating := handler.Settings["repeating"]
		log.Debug("Repeating: ", repeating)
		if repeating == "false" {
			if handler.GetStringSetting("startImmediate") == "true" {
				//				if handler.Settings["startImmediate"] == "true" {
				t.Execute(ctx, handler)
				//				t.RunAction(handler)
			} else {
				t.scheduleOnce(ctx, handler)
			}
		} else if repeating == "true" {
			t.scheduleRepeating(ctx, handler)
		} else {
			log.Error("No match for repeating: ", repeating)
		}
		log.Debugf("Settings repeating: %v", repeating)
		log.Debugf("Processing Handler: %v", handler)
	}

	return nil
}

// Start implements ext.Trigger.Start
func (t *TimerTrigger) Start() error {

	log.Debug("Trigger Start Called")
	/*	t.timers = make(map[string]*scheduler.Job)
		handlers := t.config.Handlers

		log.Debug("Processing handlers")
		for _, handler := range handlers {
			repeating := handler.Settings["repeating"]
			log.Debug("Repeating: ", repeating)
			if repeating == "false" {
				if handler.Settings["startImmediate"] == "true" {
					t.RunAction(handler)
				} else {
					t.scheduleOnce(handler)
				}
			} else if repeating == "true" {
				t.scheduleRepeating(handler)
			} else {
				log.Error("No match for repeating: ", repeating)
			}
			log.Debug("Settings repeating: ", handler.Settings["repeating"])
			log.Debugf("Processing Handler: %s", handler.ActionId)
		}*/

	return nil
}

// Stop implements ext.Trigger.Stop
func (t *TimerTrigger) Stop() error {

	log.Debug("Stopping trigger")
	for k, v := range t.timers {
		if t.timers[k].IsRunning() {
			log.Debug("Stopping timer for : ", k)
			v.Quit <- true
		} else {
			log.Debugf("Timer: %s is not running", k)
		}
	}
	return nil
}

func (t *TimerTrigger) scheduleOnce(ctx trigger.InitContext, handler *trigger.Handler) {
	log.Info("Scheduling a run one time job")

	seconds := getInitialStartInSeconds(handler)
	log.Debug("Seconds till trigger fires: ", seconds)
	timerJob := scheduler.Every(int(seconds))

	if timerJob == nil {
		log.Error("timerJob is nil")
	}

	fn := func() {
		log.Debug("-- Starting \"Once\" timer process")
		t.Execute(ctx, handler)
		//		t.RunAction(handlerCfg)
		timerJob.Quit <- true
	}

	timerJob, err := timerJob.Seconds().NotImmediately().Run(fn)
	if err != nil {
		log.Error("Error scheduleOnce flo err: ", err.Error())
	}

	//TODO: Fix these timers if needed
	//t.timers[handlerCfg.ActionId] = timerJob
}

func (t *TimerTrigger) scheduleRepeating(ctx trigger.InitContext, handler *trigger.Handler) {
	log.Info("Scheduling a repeating job")

	fn1 := func() {
		fn2_2 := func() {
			log.Debug("-- Starting \"Repeating\" (repeat) timer action")
			t.Execute(ctx, handler)
			//t.RunAction(handlerCfg)
		}
		t.scheduleJobEverySecond(handler, fn2_2)
	}

	if handler.GetStringSetting("startImmediate") == "true" {
		fn2 := func() {
			log.Debug("-- Starting \"Repeating\" (repeat) timer action")
			t.Execute(ctx, handler)
			//t.RunAction(handlerCfg)
		}
		t.scheduleJobEverySecond(handler, fn2)
	} else {
		seconds := getInitialStartInSeconds(handler)
		log.Debug("Seconds till trigger fires: ", seconds)
		timerJob := scheduler.Every(seconds)
		if timerJob == nil {
			log.Error("timerJob is nil")
		}
		timerJob, err := timerJob.Seconds().NotImmediately().Run(fn1)
		if err != nil {
			log.Error("Error scheduleRepeating (first) flo err: ", err.Error())
		}
		if timerJob == nil {
			log.Error("timerJob is nil")
		}
		// TODO Fix timers if needed
		//t.timers[handlerCfg.ActionId] = timerJob

	}
}

func getInitialStartInSeconds(handler *trigger.Handler) int {

	if _, ok := handler.GetSetting("startDate"); !ok {
		return 0
	}

	layout := time.RFC3339
	startDate := handler.GetStringSetting("startDate")
	idx := strings.LastIndex(startDate, "Z")
	timeZone := startDate[idx+1 : len(startDate)]
	log.Debug("Time Zone: ", timeZone)
	startDate = strings.TrimSuffix(startDate, timeZone)
	log.Debug("startDate: ", startDate)

	// is timezone negative
	var isNegative bool
	isNegative = strings.HasPrefix(timeZone, "-")
	// remove sign
	timeZone = strings.TrimPrefix(timeZone, "-")

	triggerDate, err := time.Parse(layout, startDate)
	if err != nil {
		log.Error("Error parsing time err: ", err.Error())
	}
	log.Debug("Time parsed from settings: ", triggerDate)

	var hour int
	var minutes int

	sliceArray := strings.Split(timeZone, ":")
	if len(sliceArray) != 2 {
		log.Error("Time zone has wrong format: ", timeZone)
	} else {
		hour, _ = strconv.Atoi(sliceArray[0])
		minutes, _ = strconv.Atoi(sliceArray[1])

		log.Debug("Duration hour: ", time.Duration(hour)*time.Hour)
		log.Debug("Duration minutes: ", time.Duration(minutes)*time.Minute)
	}

	hours, _ := strconv.Atoi(timeZone)
	log.Debug("hours: ", hours)
	if isNegative {
		log.Debug("Adding to triggerDate")
		triggerDate = triggerDate.Add(time.Duration(hour) * time.Hour)
		triggerDate = triggerDate.Add(time.Duration(minutes) * time.Minute)
	} else {
		log.Debug("Subtracting to triggerDate")
		triggerDate = triggerDate.Add(time.Duration(hour*-1) * time.Hour)
		triggerDate = triggerDate.Add(time.Duration(minutes*-1) * time.Minute)
	}

	currentTime := time.Now().UTC()
	log.Debug("Current time: ", currentTime)
	log.Debug("Setting start time: ", triggerDate)
	duration := time.Since(triggerDate)

	return int(math.Abs(duration.Seconds()))
}

type PrintJob struct {
	Msg string
}

func (j *PrintJob) Run() error {
	log.Debug(j.Msg)
	return nil
}

func (t *TimerTrigger) scheduleJobEverySecond(handler *trigger.Handler, fn func()) {

	var interval int
	if seconds := handler.GetStringSetting("seconds"); seconds != "" {
		seconds, _ := strconv.Atoi(seconds)
		interval = interval + seconds
	}
	if minutes := handler.GetStringSetting("minutes"); minutes != "" {
		minutes, _ := strconv.Atoi(minutes)
		interval = interval + minutes*60
	}
	if hours := handler.GetStringSetting("hours"); hours != "" {
		//	if hours := handlerCfg.GetSetting("hours"); hours != "" {
		hours, _ := strconv.Atoi(hours)
		interval = interval + hours*3600
	}

	log.Debug("Repeating seconds: ", interval)
	// schedule repeating
	timerJob, err := scheduler.Every(interval).Seconds().Run(fn)
	if err != nil {
		log.Error("Error scheduleRepeating (repeat seconds) flo err: ", err.Error())
	}
	if timerJob == nil {
		log.Error("timerJob is nil")
	}

	//TODO: Fix timers if needed
	//t.timers["r:"+handlerCfg.ActionId] = timerJob
}

func (t *TimerTrigger) Execute(ctx trigger.InitContext, handler *trigger.Handler) {
	log.Debug("Starting process")

	triggerData := map[string]interface{}{
		"params":      &handler,
		"triggerTime": time.Now().String(),
	}

	response, err := handler.Handle(context.Background(), triggerData)

	if err != nil {
		log.Error("Error starting action: ", err.Error())
	} else {
		log.Debugf("Action call successful: %v", response)
	}
}

/*
func (t *TimerTrigger) RunAction(handlerCfg *trigger.HandlerConfig) {
	log.Debug("Starting Immediate \"Once\" process")
	req := t.constructStartRequest(handlerCfg)
	startAttrs, _ := t.metadata.OutputsToAttrs(req.Data, false)
	action := action.Get(handlerCfg.ActionId)
	context := trigger.NewContext(context.Background(), startAttrs)
	//log.Debugf("Found action: '%+x'", action)
	log.Debugf("ActionID: '%s'", handlerCfg.ActionId)
	_, _, err := t.runner.Run(context, action, handlerCfg.ActionId, nil)

	if err != nil {
		log.Error("Error starting action: ", err.Error())
	}
}

func (t *TimerTrigger) constructStartRequest(handlerCfg *trigger.HandlerConfig) *StartRequest {

	log.Debug("Received contstruct start request")

	//TODO how to handle reply to, reply feature
	req := &StartRequest{}
	data := make(map[string]interface{})
	data["params"] = &handlerCfg
	data["triggerTime"] = time.Now().String()
	req.Data = data
	return req
}

// StartRequest describes a request for starting a ProcessInstance
type StartRequest struct {
	ProcessURI string                 `json:"flowUri"`
	Data       map[string]interface{} `json:"data"`
	ReplyTo    string                 `json:"replyTo"`
}


func GetSettingSafe(handlerCfg *trigger.HandlerConfig, setting string, defaultValue string) string {
	var retString string
	defer func() {
		if r := recover(); r != nil {
			retString = defaultValue
		}
	}()
	retString = handlerCfg.GetSetting(setting)
	return retString
}
*/