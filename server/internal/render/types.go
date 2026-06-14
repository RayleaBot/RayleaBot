package render

import (
	renderplugins "github.com/RayleaBot/RayleaBot/server/internal/render/plugins"
	renderservice "github.com/RayleaBot/RayleaBot/server/internal/render/service"
)

type Runner = renderservice.Runner
type ChromiumOptions = renderservice.ChromiumOptions
type Options = renderservice.Options
type RuntimeConfig = renderservice.RuntimeConfig
type MetricsObserver = renderservice.MetricsObserver

type Request = renderservice.Request
type PluginContext = renderservice.PluginContext
type Document = renderservice.Document
type Result = renderservice.Result
type PreviewHTML = renderservice.PreviewHTML
type Artifact = renderservice.Artifact
type TemplateAsset = renderservice.TemplateAsset

type TemplateDraft = renderservice.TemplateDraft
type TemplateSource = renderservice.TemplateSource
type TemplateFiles = renderservice.TemplateFiles
type TemplateValidationStatus = renderservice.TemplateValidationStatus
type TemplateSourceInfo = renderservice.TemplateSourceInfo
type TemplateVersion = renderservice.TemplateVersion
type TemplateSummary = renderservice.TemplateSummary
type TemplateDetail = renderservice.TemplateDetail
type TemplateValidationIssue = renderservice.TemplateValidationIssue
type TemplateValidationResult = renderservice.TemplateValidationResult
type PluginTemplateSource = renderplugins.Source
