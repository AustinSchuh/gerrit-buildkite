package main

// Structure definitions for gerrit events.

type Approval struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Value       string `json:"value"`
	OldValue    string `json:"oldValue"`
}

type User struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
}

type PatchSet struct {
	Number         int      `json:"number"`
	Revision       string   `json:"revision"`
	Parents        []string `json:"parents"`
	Ref            string   `json:"ref"`
	Uploader       User     `json:"uploader"`
	CreatedOn      int      `json:"createdOn"`
	Author         User     `json:"author"`
	Kind           string   `json:"kind,omitempty"`
	SizeInsertions int      `json:"sizeInsertions,omitempty"`
	SizeDeletions  int      `json:"sizeDeletions,omitempty"`
}

type Change struct {
	Project              string   `json:"project"`
	Branch               string   `json:"branch"`
	ID                   string   `json:"id"`
	Number               int      `json:"number"`
	Subject              string   `json:"subject"`
	Owner                User     `json:"owner"`
	Assignee             *User    `json:"assignee,omitempty"`
	URL                  string   `json:"url"`
	CommitMessage        string   `json:"commitMessage"`
	CherryPickOfChange   int      `json:"cherryPickOfChange,omitempty"`
	CherryPickOfPatchSet int      `json:"cherryPickOfPatchSet,omitempty"`
	CreatedOn            int      `json:"createdOn"`
	Status               string   `json:"status"`
	Wip                  bool     `json:"wip,omitempty"`
	Topic                string   `json:"topic,omitempty"`
	Private              bool     `json:"private,omitempty"`
	Hashtags             []string `json:"hashtags,omitempty"`
}

type ChangeKey struct {
	ID string `json:"id"`
}

type RefUpdate struct {
	OldRev  string `json:"oldRev"`
	NewRev  string `json:"newRev"`
	RefName string `json:"refName"`
	Project string `json:"project"`
}

// Event represents a Gerrit event.
type Event struct {
	Abandoner *User `json:"abandoner,omitempty"`
	// Author         *User      `json:"author"`
	Uploader       *User      `json:"uploader"`
	Reviewer       *User      `json:"reviewer"`
	Adder          *User      `json:"adder"`
	Remover        *User      `json:"remover"`
	Submitter      *User      `json:"submitter,omitempty"`
	NewRev         string     `json:"newRev,omitempty"`
	Ref            string     `json:"ref,omitempty"`
	TargetNode     string     `json:"targetNode,omitempty"`
	TargetUri      string     `json:"targetUri,omitempty"`
	Approvals      []Approval `json:"approvals,omitempty"`
	Comment        string     `json:"comment,omitempty"`
	PatchSet       PatchSet   `json:"patchSet"`
	Change         Change     `json:"change"`
	Project        string     `json:"project"`
	RefName        string     `json:"refName"`
	ChangeKey      ChangeKey  `json:"changeKey"`
	RefUpdate      RefUpdate  `json:"refUpdate"`
	Type           string     `json:"type"`
	Reason         string     `json:"reason,omitempty"`
	EventCreatedOn int        `json:"eventCreatedOn"`
	Status         string     `json:"status,omitempty"`
	RefStatus      string     `json:"refStatus,omitempty"`
	NodesCount     int        `json:"nodesCount,omitempty"`
	OldTopic       string     `json:"oldTopic,omitempty"`
	Changer        *User      `json:"changer,omitempty"`
	Editor         *User      `json:"editor,omitempty"`
	Restorer       *User      `json:"restorer,omitempty"`
	OldAssignee    *User      `json:"oldAssignee,omitempty"`
	Added          []string   `json:"added,omitempty"`
	Removed        []string   `json:"removed,omitempty"`
	Hashtags       []string   `json:"hashtags,omitempty"`
}
