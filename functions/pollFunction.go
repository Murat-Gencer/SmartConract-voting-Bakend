package functions

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"voting-backend/models"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/near/borsh-go"
)


// Global variables
var (
	client    *rpc.Client
	from      solana.PrivateKey
	programID solana.PublicKey
	database  *sql.DB // Assuming you have this defined elsewhere
)

const (
	DatabaseUser     = "admin"
	DatabasePassword = "admin"
	DatabaseName     = "voting_db"
	DatabaseHost     = "localhost"
	DatabasePort     = 5432
	ServerPort       = ":8080"
)

func Init() {
	connectionString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		DatabaseHost, DatabasePort, DatabaseUser, DatabasePassword, DatabaseName,
	)

	var err error
	database, err = sql.Open("postgres", connectionString)
	if err != nil {
		log.Fatal("Failed to open database connection:", err)
	}
	log.Println("Database connection established successfully")
	client = rpc.New(rpc.DevNet_RPC)

	keyBytes := []byte{145, 161, 48, 78, 178, 205, 91, 6, 14, 94, 134, 79, 116, 140, 40, 112, 43, 73, 114, 196, 22, 163, 127, 230, 229, 67, 98, 59, 255, 70, 117, 135, 132, 102, 75, 98, 50, 242, 51, 46, 58, 13, 28, 4, 123, 75, 207, 44, 250, 218, 101, 71, 134, 12, 120, 167, 186, 208, 13, 27, 24, 83, 133, 120}
	from = solana.PrivateKey(keyBytes)

	programID = solana.MustPublicKeyFromBase58("GW1r76tkZDNpdKf7BD7ap1EtPvnQb592apWuaKWCyckd")
}

func generatePollId(question, optionsStr string, creator solana.PublicKey) uint64 {
	hasher := sha256.New()
	hasher.Write([]byte(question))
	hasher.Write([]byte(optionsStr))
	hasher.Write(creator.Bytes()) // Creator'ı da ekle (farklı creator'lar farklı ID alır)

	hash := hasher.Sum(nil)
	pollId := binary.LittleEndian.Uint64(hash[:8]) // İlk 8 byte'ı uint64'e çevir

	return pollId
}
func CreatePoll(c *gin.Context) {
	question := c.PostForm("Question")
	optionsStr := c.PostForm("Options")
	creator := c.PostForm("Creator")

	if question == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Question is required",
		})
		return
	}

	if creator == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Creator is required",
		})
		return
	}

	var options []string
	if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid options format: " + err.Error(),
		})
		return
	}

	if len(options) < 2 {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Poll must have at least 2 options",
		})
		return
	}

	if len(options) > 10 {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Poll cannot have more than 10 options",
		})
		return
	}

	if len(question) > 200 {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Question cannot be longer than 200 characters",
		})
		return
	}

	var imagePath string
	file, err := c.FormFile("Image")
	if err == nil {
		imagePath = "uploads/" + file.Filename
		if err := c.SaveUploadedFile(file, imagePath); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{
				Success: false,
				Error:   "Failed to save image: " + err.Error(),
			})
			return
		}
	} else if err != http.ErrMissingFile {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Error reading image file: " + err.Error(),
		})
		return
	}
	pollId := generatePollId(question, options[0]+options[1]+options[2]+options[3], from.PublicKey())

	sig, err := sendCreatePollTransaction(c.Request.Context(), client, from, question, options, pollId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to create poll on blockchain: " + err.Error(),
		})
		return
	}

	authorityPubKey := from.PublicKey()

	pollIdBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pollIdBytes, pollId)

	seeds := [][]byte{
		[]byte("poll"),
		authorityPubKey.Bytes(),
		pollIdBytes,
	}

	pollPubKey, _, err := solana.FindProgramAddress(seeds, programID)

	createdAt := time.Now().UTC()
	query := `
		INSERT INTO polls (question, options, creator, created_at, signature, image_path, poll_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	err = database.QueryRow(query, question, pq.Array(options), creator, createdAt, sig.String(), imagePath, pollPubKey.String()).Scan(&pollId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to save poll to database: " + err.Error(),
		})
		return
	}

	response := models.CreatePollResponse{
		ID:          int(pollId),
		Signature:   sig.String(),
		PollAddress: pollPubKey.String(),
		Explorer:    fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig.String()),
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    response,
	})
}

func sendCreatePollTransaction(ctx context.Context, client *rpc.Client, authority solana.PrivateKey, question string, options []string, pollId uint64) (solana.Signature, error) {
	authorityPubKey := authority.PublicKey()

	pollIdBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(pollIdBytes, pollId)

	seeds := [][]byte{
		[]byte("poll"),
		authorityPubKey.Bytes(),
		pollIdBytes,
	}

	pollPubKey, bump, err := solana.FindProgramAddress(seeds, programID)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to find program address: %w", err)
	}

	log.Printf("Generated poll PDA: %s with bump: %d", pollPubKey.String(), bump)

	createPollDiscriminator := []byte{182, 171, 112, 238, 6, 219, 14, 110}

	duration := int64(86400) // 24 hours

	args := models.CreatePollArgs{
		PollId:   pollId,
		Question: question,
		Options:  options,
		Duration: duration,
	}

	data, err := borsh.Serialize(args)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to serialize args: %w", err)
	}

	instructionData := append(createPollDiscriminator, data...)

	accounts := solana.AccountMetaSlice{
		{PublicKey: pollPubKey, IsSigner: false, IsWritable: true},
		{PublicKey: authorityPubKey, IsSigner: true, IsWritable: true},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
	}

	instruction := solana.NewInstruction(programID, accounts, instructionData)

	recentBlockhashResp, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to get recent blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		recentBlockhashResp.Value.Blockhash,
		solana.TransactionPayer(authorityPubKey),
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if authorityPubKey.Equals(key) {
			return &authority
		}
		return nil
	})

	sig, err := client.SendTransaction(ctx, tx)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	log.Printf("Transaction sent successfully. Signature: %s", sig.String())
	log.Printf("Poll PDA: %s", pollPubKey.String())

	return sig, nil
}

func sendCastVoteTransaction(ctx context.Context, client *rpc.Client, authority solana.PrivateKey, pollAddress string, optionIndex uint8) (solana.Signature, error) {
	authorityPubKey := authority.PublicKey()
	pollPubKey := solana.MustPublicKeyFromBase58(pollAddress)

	voterSeeds := [][]byte{
		[]byte("voter"),
		pollPubKey.Bytes(),
		authorityPubKey.Bytes(),
	}
	voterRecordPubKey, _, err := solana.FindProgramAddress(voterSeeds, programID)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to derive voter record PDA: %w", err)
	}

	castVoteDiscriminator := []byte{20, 212, 15, 189, 69, 180, 69, 151}

	args := models.CastVoteArgs{OptionIndex: optionIndex}
	data, err := borsh.Serialize(args)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to serialize cast vote args: %w", err)
	}

	instructionData := append(castVoteDiscriminator, data...)

	accounts := solana.AccountMetaSlice{
		{PublicKey: pollPubKey, IsSigner: false, IsWritable: true},
		{PublicKey: voterRecordPubKey, IsSigner: false, IsWritable: true},
		{PublicKey: authorityPubKey, IsSigner: true, IsWritable: true},
		{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
	}

	instruction := solana.NewInstruction(programID, accounts, instructionData)

	blockhashResp, err := client.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to fetch recent blockhash: %w", err)
	}

	tx, err := solana.NewTransaction(
		[]solana.Instruction{instruction},
		blockhashResp.Value.Blockhash,
		solana.TransactionPayer(authorityPubKey),
	)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to create transaction: %w", err)
	}

	tx.Sign(func(pubkey solana.PublicKey) *solana.PrivateKey {
		if authorityPubKey.Equals(pubkey) {
			return &authority
		}
		return nil
	})

	sig, err := client.SendTransaction(ctx, tx)
	if err != nil {
		return solana.Signature{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	log.Printf("Vote cast successfully. Signature: %s", sig.String())
	return sig, nil
}

func CastVote(c *gin.Context) {
	id := c.Param("id")
	var req models.CastVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid request: " + err.Error(),
		})
		return
	}
	voterPubKey, err := solana.PublicKeyFromBase58(req.VoterWallet)
	if err != nil {
		log.Printf("Failed to send vote transaction: %v", voterPubKey)
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Error:   "Invalid voter wallet address",
		})
		return
	}

	var poll struct {
		ID          int
		Question    string
		Options     []string
		PollAddress string
	}

	pollQuery := `SELECT id, question, options, poll_address FROM polls WHERE id = $1`

	err = database.QueryRow(pollQuery, id).Scan(
		&poll.ID,
		&poll.Question,
		pq.Array(&poll.Options),
		&poll.PollAddress,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, models.APIResponse{
				Success: false,
				Error:   "Poll not found",
			})
			return
		}
		log.Printf("Database error checking poll: %v", err)
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Database error",
		})
		return
	}

	var existingVoteID int
	checkVoteQuery := `SELECT id FROM votes WHERE poll_id = $1 AND voter_address = $2`
	err = database.QueryRow(checkVoteQuery, poll.ID, req.VoterWallet).Scan(&existingVoteID)

	if err == nil {
		c.JSON(http.StatusOK, models.APIResponse{
			Success: false,
			Error:   "You have already voted this poll.",
		})
		return
	} else if err != sql.ErrNoRows {
		log.Printf("Database error checking existing vote: %v", err)
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Database error",
		})
		return
	}

	sig, err := sendCastVoteTransaction(c.Request.Context(), client, from,
		poll.PollAddress, uint8(req.OptionIndex))
	if err != nil {
		log.Printf("Failed to send vote transaction: %v", sig)
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to submit vote to blockchain",
		})
		return
	}

	votedAt := time.Now().UTC()
	query := `
		INSERT INTO votes (poll_id, voter_address, option_index, vote_signature, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = database.Exec(query, poll.ID, req.VoterWallet, req.OptionIndex, sig.String(), votedAt)
	if err != nil {
		log.Printf("Failed to save vote to database: %v", err)
	}

	response := map[string]interface{}{
		"signature":    sig.String(),
		"explorer_url": fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", sig.String()),
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    response,
	})
}

func ListPolls(c *gin.Context) {
	query := `SELECT * FROM polls ORDER BY id DESC`
	rows, err := database.Query(query)

	if err != nil {
		log.Printf("Failed to fetch polls: %v", err)
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to retrieve polls",
		})
		return
	}
	defer rows.Close()

	var polls []models.Poll
	for rows.Next() {
		var poll models.Poll
		var options []string

		if err := rows.Scan(
			&poll.ID,
			&poll.Question,
			pq.Array(&options),
			&poll.Creator,
			&poll.CreatedAt,
			&poll.Signature,
			&poll.ImagePath,
			&poll.PollAddress,
		); err != nil {
			log.Printf("Failed to scan poll row: %v", err)
			continue
		}

		poll.Options = options
		polls = append(polls, poll)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating over poll rows: %v", err)
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Error:   "Failed to process polls",
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    polls,
	})
}

func GetUserVotes(c *gin.Context) {
	var req struct {
		WalletAddress string `json:"walletAddress"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.WalletAddress == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Success: false,
			Message: "Invalid request: walletAddress is required",
		})
		return
	}

	rows, err := database.Query("SELECT poll_id, option_index FROM votes WHERE voter_address=$1", req.WalletAddress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Success: false,
			Message: "DB error",
			Error:   err.Error(),
		})
		return
	}
	defer rows.Close()

	var userVotes []models.UserVote
	for rows.Next() {
		var vote models.UserVote
		if err := rows.Scan(&vote.PollID, &vote.Option); err != nil {
			c.JSON(http.StatusInternalServerError, models.APIResponse{
				Success: false,
				Message: "DB error",
				Error:   err.Error(),
			})
			return
		}
		userVotes = append(userVotes, vote)
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Success: true,
		Data:    map[string]interface{}{"userVotes": userVotes},
	})
}
