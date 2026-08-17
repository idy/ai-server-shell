// Package backend defines protocol-neutral application contracts shared by
// every protocol handler exposed by AI Server Shell.
package backend

// Capability identifies an independently injectable backend surface.
type Capability string

const (
	CapabilityResponses     Capability = "responses"
	CapabilityChat          Capability = "chat"
	CapabilityCompletions   Capability = "completions"
	CapabilityEmbeddings    Capability = "embeddings"
	CapabilityAudio         Capability = "audio"
	CapabilityImages        Capability = "images"
	CapabilityVideos        Capability = "videos"
	CapabilityModerations   Capability = "moderations"
	CapabilityModels        Capability = "models"
	CapabilityFiles         Capability = "files"
	CapabilityUploads       Capability = "uploads"
	CapabilityBatches       Capability = "batches"
	CapabilityAssistants    Capability = "assistants"
	CapabilityVectorStores  Capability = "vector_stores"
	CapabilityFineTuning    Capability = "fine_tuning"
	CapabilityEvals         Capability = "evals"
	CapabilityContainers    Capability = "containers"
	CapabilitySkills        Capability = "skills"
	CapabilityChatKit       Capability = "chatkit"
	CapabilityOrganization  Capability = "organization"
	CapabilityConversations Capability = "conversations"
	CapabilityRealtime      Capability = "realtime"
)

// Capabilities is the complete frozen M1 capability inventory.
var Capabilities = [...]Capability{
	CapabilityResponses,
	CapabilityChat,
	CapabilityCompletions,
	CapabilityEmbeddings,
	CapabilityAudio,
	CapabilityImages,
	CapabilityVideos,
	CapabilityModerations,
	CapabilityModels,
	CapabilityFiles,
	CapabilityUploads,
	CapabilityBatches,
	CapabilityAssistants,
	CapabilityVectorStores,
	CapabilityFineTuning,
	CapabilityEvals,
	CapabilityContainers,
	CapabilitySkills,
	CapabilityChatKit,
	CapabilityOrganization,
	CapabilityConversations,
	CapabilityRealtime,
}
