{{/*
Expand the name of the chart.
*/}}
{{- define "operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "operator.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "operator.labels" -}}
helm.sh/chart: {{ include "operator.chart" . }}
{{ include "operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Allow Operator image tag to be overridden. */}}
{{- define "operator.imageTag" -}}
  {{- default .Chart.Version .Values.operator.tag -}}
{{- end -}}

{{/* Allow LocalDiskManager image tag to be overridden. */}}
{{- define "hwameistor.localDiskManagerImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.localDiskManager.manager.tag -}}
{{- end -}}

{{/* Allow LocalStorage image tag to be overridden. */}}
{{- define "hwameistor.localStorageImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.localStorage.member.tag -}}
{{- end -}}

{{/* Allow Scheduler image tag to be overridden. */}}
{{- define "hwameistor.schedulerImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.scheduler.tag -}}
{{- end -}}

{{/* Allow Admission image tag to be overridden. */}}
{{- define "hwameistor.admissionImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.admission.tag -}}
{{- end -}}

{{/* Allow Evictor image tag to be overridden. */}}
{{- define "hwameistor.evictorImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.evictor.tag -}}
{{- end -}}

{{/* Allow Apiserver image tag to be overridden. */}}
{{- define "hwameistor.apiserverImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.apiserver.tag -}}
{{- end -}}

{{/* Allow Exporter image tag to be overridden. */}}
{{- define "hwameistor.exporterImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.exporter.tag -}}
{{- end -}}

{{/* Allow UI image tag to be overridden. */}}
{{- define "hwameistor.uiImageTag" -}}
  {{- default .Values.global.hwameistorVersion .Values.ui.tag -}}
{{- end -}}
