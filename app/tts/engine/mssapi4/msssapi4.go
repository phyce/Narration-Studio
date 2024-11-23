package mssapi4

import (
	"bufio"
	"fmt"
	"nstudio/app/common/audio"
	"nstudio/app/common/issue"
	"nstudio/app/common/process"
	"nstudio/app/common/response"
	"nstudio/app/common/util"
	"nstudio/app/common/util/fileIndex"
	"nstudio/app/config"
	"nstudio/app/tts/engine"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

func (sapi *MsSapi4) Initialize() error {
	return nil
}

func (sapi *MsSapi4) Start(modelName string) error {
	return nil
}

func (sapi *MsSapi4) Stop(modelName string) error {
	return nil
}

func (sapi *MsSapi4) Play(message util.CharacterMessage) error {
	//Just Generate() , play audio and then delete
	response.Debug(response.Data{
		Summary: "Ms Sapi 4 playing:" + message.Character,
		Detail:  message.Text,
	})

	audioClip, err := sapi.Generate(message.Voice.Voice, []byte(message.Text))
	if err != nil {
		return issue.Trace(err)
	}

	audio.PlayRawAudioBytes(audioClip)
	response.Debug(response.Data{
		Summary: "Finshed playing audio for:" + message.Character,
		Detail:  message.Text,
	})
	return nil
}

func (sapi *MsSapi4) Save(messages []util.CharacterMessage, play bool) error {
	response.Debug(response.Data{
		Summary: "Ms Sapi 4 saving messages",
	})

	err, outputPath := util.ExpandPath(config.GetSettings().OutputPath)
	if err != nil {
		return issue.Trace(err)
	}

	if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
		return issue.Trace(fmt.Errorf("Failed to create output directory: %w", err))
	}

	for _, message := range messages {
		outputFilename := util.GenerateFilename(
			message,
			fileIndex.Get(),
			outputPath,
		)

		audioClip, err := sapi.Generate(message.Voice.Voice, []byte(message.Text))
		if err != nil {
			return issue.Trace(err)
		}

		err = os.WriteFile(outputFilename, audioClip, 0644)
		if err != nil {
			return issue.Trace(fmt.Errorf("Failed to write audio to file '%s': %w", outputFilename, err))
		}

		if play {
			audio.PlayRawAudioBytes(audioClip)
		}
	}

	return nil
}

func (sapi *MsSapi4) Generate(voice string, payload []byte) ([]byte, error) {

	command := exec.Command(
		config.GetEngine().Local.MsSapi4.Location,
		voice,
		strconv.Itoa(config.GetEngine().Local.MsSapi4.Pitch),
		strconv.Itoa(config.GetEngine().Local.MsSapi4.Speed),
		string(payload),
	)

	if !config.Debug() {
		process.HideCommandLine(command)
	}

	command.Dir = config.Get().Settings.OutputPath

	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to get stdout pipe: %w", err))
	}

	if err := command.Start(); err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to start sapi4out.exe: %w", err))
	}

	scanner := bufio.NewScanner(stdoutPipe)

	var filename string

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, issue.Trace(err)
		}
		return nil, issue.Trace(fmt.Errorf("no output received from sapi4out.exe"))
	}

	filename = scanner.Text()
	response.Debug(response.Data{
		Summary: "Ms Sapi 4 generated:" + filename,
		Detail:  string(payload),
	})

	if err := command.Wait(); err != nil {
		return nil, issue.Trace(fmt.Errorf("sapi4out.exe execution failed: %w", err))
	}

	audioFilePath := filepath.Join(command.Dir, filename)
	fmt.Println("Audio File Path:", audioFilePath)

	audioBytes, err := os.ReadFile(audioFilePath)
	if err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to read audio file: %w", err))
	}

	if err := os.Remove(audioFilePath); err != nil {
		return nil, issue.Trace(fmt.Errorf("failed to delete audio file: %w", err))
	}

	return audioBytes, nil
}

func (sapi *MsSapi4) GetVoices(model string) ([]engine.Voice, error) {
	return []engine.Voice{
		{ID: "sam", Name: "Sam", Gender: "Male"},

		{ID: "mary", Name: "Mary", Gender: "Female"},
		{ID: "maryphone", Name: "Mary (for Telephone)", Gender: "Female"},
		{ID: "maryhall", Name: "Mary in Hall", Gender: "Female"},
		{ID: "marystadium", Name: "Mary in Stadium", Gender: "Female"},
		{ID: "maryspace", Name: "Mary in Space", Gender: "Female"},

		{ID: "mike", Name: "Mike", Gender: "Male"},
		{ID: "mikephone", Name: "Mike (for Telephone)", Gender: "Male"},
		{ID: "mikehall", Name: "Mike in Hall", Gender: "Male"},
		{ID: "mikestadium", Name: "Mike in Stadium", Gender: "Male"},
		{ID: "mikespace", Name: "Mike in Space", Gender: "Male"},

		{ID: "robo1", Name: "RoboSoft One", Gender: ""},
		{ID: "robo2", Name: "RoboSoft Two", Gender: ""},
		{ID: "robo3", Name: "RoboSoft Three", Gender: ""},
		{ID: "robo4", Name: "RoboSoft Four", Gender: ""},
		{ID: "robo5", Name: "RoboSoft Five", Gender: ""},
		{ID: "robo6", Name: "RoboSoft Six", Gender: ""},

		{ID: "whisperfemale", Name: "Female Whisper", Gender: "Female"},
		{ID: "whispermale", Name: "Male Whisper", Gender: "Male"},

		{ID: "trueman1", Name: "Adult Male #1, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman2", Name: "Adult Male #2, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman3", Name: "Adult Male #3, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman4", Name: "Adult Male #4, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman5", Name: "Adult Male #5, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman6", Name: "Adult Male #6, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman7", Name: "Adult Male #7, American English (TruVoice)", Gender: "Male"},
		{ID: "trueman8", Name: "Adult Male #8, American English (TruVoice)", Gender: "Male"},

		{ID: "truefemale1", Name: "Adult Female #1, American English (TruVoice)", Gender: "Female"},
		{ID: "truefemale2", Name: "Adult Female #2, American English (TruVoice)", Gender: "Female"},
	}, nil
}

func (sapi *MsSapi4) FetchModels() map[string]engine.Model {
	return map[string]engine.Model{
		"mssapi4": {
			ID:     "mssapi4",
			Name:   "MS Speech API 4",
			Engine: "mssapi4",
		},
	}
}
