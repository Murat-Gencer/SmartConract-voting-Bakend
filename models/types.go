package models

import "time"

type Poll struct {
	ID          int       `db:"id" json:"id"`
	Question    string    `db:"question" json:"question"`
	Options     []string  `db:"options" json:"options"`
	Creator     string    `db:"creator" json:"creator"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	Signature   string    `db:"signature" json:"signature"`
	ImagePath   *string   `db:"image_path" json:"image_path"`
	PollAddress string    `db:"poll_address" json:"poll_address"`
}

type Vote struct {
	ID             int       `json:"id" db:"id"`
	PollID         int       `json:"poll_id" db:"poll_id"`
	WalletAddress  string    `json:"wallet_address" db:"wallet_address" binding:"required"`
	SelectedOption int       `json:"selected_option" db:"selected_option" binding:"required"`
	TransactionID  string    `json:"transaction_id" db:"transaction_id" binding:"required"`
	VoteTimestamp  time.Time `json:"vote_timestamp" db:"vote_timestamp"`
}



type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}


type CastVoteArgs struct {
	OptionIndex uint8 `borsh:"option_index"`
}
type CastVoteRequest struct {
	OptionIndex int    `json:"option_index" binding:"required"`
	VoterWallet string `json:"voter_wallet" binding:"required"`
}
type CreatePollArgs struct {
	PollId   uint64   `borsh:"poll_id"`
	Question string   `borsh:"question"`
	Options  []string `borsh:"options"`
	Duration int64    `borsh:"duration"`
}


type CreatePollResponse struct {
	ID          int    `json:"id"`
	Signature   string `json:"signature"`
	PollAddress string `json:"poll_address"`
	Explorer    string `json:"explorer_url"`
}

type UserVote struct {
	PollID int    `json:"pollId"`
	Option string `json:"option"`
}
