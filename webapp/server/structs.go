package main

type TaskData struct {
	Title  string
	Body   string
	Done   bool
	Answer string
}

type UserData struct {
	Username string
	Tasks    []TaskData
}

type MainData struct {
	Username string
	Users    []UserData
	Tasks    []TaskData
	Empty    bool
}

type TranslationData struct {
	Word        string
	Translation string
}

type TestData struct {
	Word string
}

type TestCheckData struct {
	Translation string
}

type ProfileData struct {
	Username   string
	Translated int
	Learned    int
	Unknown    int
}
