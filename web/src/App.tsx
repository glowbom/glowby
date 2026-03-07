import { useEffect, useMemo, useRef, useState } from 'react';
import {
  MODEL_PROVIDERS,
  decodeModelOptionValue,
  encodeModelOptionValue,
  modelCatalogGroups,
  providerDefinition,
  resolveRefineModel,
} from './lib/model-catalog';
import { lineLooksLikeMarkdown, parseConsoleHeading, parseConsoleMarkdown } from './lib/console-render';
import { openCodeApi, toErrorMessage } from './lib/api';
import { useRefineRun } from './hooks/useRefineRun';
import type {
  AuthProviderID,
  OpenCodeAvailableProvider,
  OpenAIAuthMode,
  OpenAIModelsResponseModel,
  OpenCodeIDE,
  OpenCodeAuthStatus,
  OpenCodeInstructionPickedFile,
  OpenCodeHealthResponse,
  OpenCodeProjectIDEAction,
  OpenCodeProjectIDEStatusResponse,
  OpenCodeProject,
  OpenCodeProjectEnvelope,
  OpenCodeQuestionItem,
  ProviderID,
  ProviderKeyState,
} from './types/opencode';

type CredentialMode = 'auth' | 'api-key' | 'opencode-config';
const OPENCODE_DEFAULT_MODEL_VALUE = '__opencode_default__';
const MAX_INSTRUCTION_ATTACHMENT_BYTES = 40 * 1024 * 1024;

const RUN_STATUS_LABEL: Record<string, string> = {
  idle: 'Idle',
  running: 'Running',
  completed: 'Completed',
  failed: 'Failed',
  cancelled: 'Cancelled',
};

const DEFAULT_PROVIDER_KEYS: ProviderKeyState = {
  openaiKey: '',
  anthropicKey: '',
  geminiKey: '',
  fireworksKey: '',
  openrouterKey: '',
  opencodeZenKey: '',
  xaiKey: '',
};

function isMultiSelect(prompt: string, inputType?: string): boolean {
  const promptLower = prompt.toLowerCase();
  const typeLower = (inputType || '').toLowerCase();

  if (typeLower.includes('multi') || typeLower.includes('checkbox')) {
    return true;
  }

  return (
    promptLower.includes('select all') ||
    promptLower.includes('choose all') ||
    promptLower.includes('select one or more') ||
    promptLower.includes('choose one or more') ||
    promptLower.includes('multiple')
  );
}

function toggleSelection(current: string[], value: string, multiSelect: boolean): string[] {
  if (!multiSelect) {
    return [value];
  }

  if (current.includes(value)) {
    return current.filter((item) => item !== value);
  }

  return [...current, value];
}

function targetSummary(project: OpenCodeProject | null): Array<{ id: string; stack: string; outputDir: string }> {
  if (!project) {
    return [];
  }

  return Object.entries(project.targets).map(([id, target]) => ({
    id,
    stack: target.stack || 'unknown',
    outputDir: target.outputDir || id,
  }));
}

function formatCredentialType(value?: string): string {
  switch ((value || '').toLowerCase()) {
    case 'oauth':
      return 'ChatGPT OAuth credential';
    case 'api':
      return 'OpenAI API credential';
    case 'none':
      return 'None';
    default:
      return 'Unknown';
  }
}

function formatAuthMode(value?: string): string {
  switch ((value || '').toLowerCase()) {
    case 'api-key':
      return 'API Key';
    case 'codex-jwt':
      return 'ChatGPT Login';
    case 'opencode-config':
      return 'OpenCode Config';
    default:
      return 'Unknown';
  }
}

function formatAttachmentSize(sizeBytes: number): string {
  if (!Number.isFinite(sizeBytes) || sizeBytes <= 0) {
    return '0 B';
  }
  if (sizeBytes < 1024) {
    return `${sizeBytes} B`;
  }

  const kib = sizeBytes / 1024;
  if (kib < 1024) {
    return `${Math.round(kib)} KB`;
  }

  const mib = kib / 1024;
  if (mib < 1024) {
    return `${mib.toFixed(1)} MB`;
  }

  return `${(mib / 1024).toFixed(1)} GB`;
}

function inferProviderFromCustomModel(customModel: string): ProviderID | null {
  const trimmed = customModel.trim();
  const slashIndex = trimmed.indexOf('/');
  if (slashIndex <= 0) {
    return null;
  }

  const providerId = trimmed.slice(0, slashIndex) as ProviderID;
  if (!providerDefinition(providerId)) {
    return null;
  }

  return providerId;
}

function defaultModelOptionValue(dynamicOpenAIModels: OpenAIModelsResponseModel[] = []): string {
  const firstGroup = modelCatalogGroups(dynamicOpenAIModels)[0];
  if (!firstGroup || firstGroup.models.length === 0) {
    return '';
  }

  const firstModel = firstGroup.models[0];
  if (!firstModel) {
    return '';
  }
  return encodeModelOptionValue(firstGroup.provider.id, firstModel.id);
}

function ideActionFor(actions: OpenCodeProjectIDEAction[], ide: OpenCodeIDE): OpenCodeProjectIDEAction | undefined {
  return actions.find((item) => item.ide === ide);
}

export default function App() {
  const [health, setHealth] = useState<OpenCodeHealthResponse | null>(null);
  const [authStatus, setAuthStatus] = useState<OpenCodeAuthStatus | null>(null);
  const [healthError, setHealthError] = useState<string | null>(null);
  const [authError, setAuthError] = useState<string | null>(null);
  const [isCheckingSetup, setIsCheckingSetup] = useState(false);

  const [projectPath, setProjectPath] = useState('');
  const [projectEnvelope, setProjectEnvelope] = useState<OpenCodeProjectEnvelope | null>(null);
  const [projectError, setProjectError] = useState<string | null>(null);
  const [isLoadingProject, setIsLoadingProject] = useState(false);
  const [isPickingFolder, setIsPickingFolder] = useState(false);
  const [folderPickerInfo, setFolderPickerInfo] = useState<string | null>(null);
  const [folderPickerWarning, setFolderPickerWarning] = useState<string | null>(null);
  const [instructionAttachments, setInstructionAttachments] = useState<OpenCodeInstructionPickedFile[]>([]);
  const [isPickingInstructionFiles, setIsPickingInstructionFiles] = useState(false);
  const [instructionPickerInfo, setInstructionPickerInfo] = useState<string | null>(null);
  const [instructionPickerWarning, setInstructionPickerWarning] = useState<string | null>(null);
  const [ideStatus, setIdeStatus] = useState<OpenCodeProjectIDEStatusResponse | null>(null);
  const [isLoadingIdeStatus, setIsLoadingIdeStatus] = useState(false);
  const [ideStatusError, setIdeStatusError] = useState<string | null>(null);
  const [ideOpenInfo, setIdeOpenInfo] = useState<string | null>(null);
  const [ideOpenError, setIdeOpenError] = useState<string | null>(null);
  const [openingIDE, setOpeningIDE] = useState<OpenCodeIDE | null>(null);

  const [providerKeys, setProviderKeys] = useState<ProviderKeyState>(DEFAULT_PROVIDER_KEYS);
  const [credentialMode, setCredentialMode] = useState<CredentialMode>('opencode-config');
  const [authProvider, setAuthProvider] = useState<AuthProviderID>('chatgpt');
  const [isUpdatingAuthConnection, setIsUpdatingAuthConnection] = useState(false);
  const [authConnectionInfo, setAuthConnectionInfo] = useState<string | null>(null);
  const [authConnectionError, setAuthConnectionError] = useState<string | null>(null);

  const [dynamicOpenAIModels, setDynamicOpenAIModels] = useState<OpenAIModelsResponseModel[]>([]);
  const [isLoadingOpenAIModels, setIsLoadingOpenAIModels] = useState(false);
  const [openAIModelsInfo, setOpenAIModelsInfo] = useState<string | null>(null);
  const [openAIModelsWarning, setOpenAIModelsWarning] = useState<string | null>(null);
  const [openCodeConfigProviders, setOpenCodeConfigProviders] = useState<OpenCodeAvailableProvider[]>([]);
  const [selectedOpenCodeModel, setSelectedOpenCodeModel] = useState(OPENCODE_DEFAULT_MODEL_VALUE);
  const [isLoadingOpenCodeModels, setIsLoadingOpenCodeModels] = useState(false);
  const [openCodeModelsInfo, setOpenCodeModelsInfo] = useState<string | null>(null);
  const [openCodeModelsWarning, setOpenCodeModelsWarning] = useState<string | null>(null);

  const [selectedModelOption, setSelectedModelOption] = useState(() => defaultModelOptionValue());
  const [customModel, setCustomModel] = useState('');
  const [instructions, setInstructions] = useState(
    'Make this project production ready. Follow AGENTS.md when present.',
  );
  const [formError, setFormError] = useState<string | null>(null);

  const [simpleAnswerText, setSimpleAnswerText] = useState('');
  const [simpleCustomAnswer, setSimpleCustomAnswer] = useState('');
  const [simpleSelectedOptions, setSimpleSelectedOptions] = useState<string[]>([]);

  const [nestedTextById, setNestedTextById] = useState<Record<string, string>>({});
  const [nestedCustomById, setNestedCustomById] = useState<Record<string, string>>({});
  const [nestedSelectionsById, setNestedSelectionsById] = useState<Record<string, string[]>>({});

  const consoleRef = useRef<HTMLDivElement | null>(null);
  const [autoScrollEnabled, setAutoScrollEnabled] = useState(true);

  const refine = useRefineRun();

  const activeProject = projectEnvelope?.project ?? null;
  const targetRows = useMemo(() => targetSummary(activeProject), [activeProject]);
  const modelGroups = useMemo(() => modelCatalogGroups(dynamicOpenAIModels), [dynamicOpenAIModels]);
  const isAuthMode = credentialMode === 'auth';
  const isApiKeyMode = credentialMode === 'api-key';
  const isOpenCodeConfigMode = credentialMode === 'opencode-config';
  const effectiveOpenAIAuthMode: OpenAIAuthMode = isAuthMode
    ? 'codex-jwt'
    : isOpenCodeConfigMode
      ? 'opencode-config'
      : 'api-key';
  const isChatGPTConnected =
    authStatus?.openaiCredentialType === 'oauth' || authStatus?.cachedGlowbomAuthMode === 'codex-jwt';
  const authProviderConnected = authProvider === 'chatgpt' && isChatGPTConnected;

  const visibleModelGroups = useMemo(() => {
    if (isAuthMode) {
      return modelGroups.filter((group) => group.provider.id === 'openai');
    }
    if (isApiKeyMode) {
      return modelGroups;
    }
    return [];
  }, [isApiKeyMode, isAuthMode, modelGroups]);

  const allOptionValues = useMemo(
    () =>
      visibleModelGroups.flatMap((group) =>
        group.models.map((model) => encodeModelOptionValue(group.provider.id, model.id)),
      ),
    [visibleModelGroups],
  );

  const decodedSelection = useMemo(() => decodeModelOptionValue(selectedModelOption), [selectedModelOption]);

  const resolvedSelection = useMemo(() => {
    if (!decodedSelection) {
      return null;
    }
    return resolveRefineModel(decodedSelection.providerId, decodedSelection.modelId);
  }, [decodedSelection]);

  const selectedProvider = resolvedSelection?.providerId || null;
  const selectedProviderDefinition = selectedProvider ? providerDefinition(selectedProvider) : undefined;
  const selectedProviderKeyField = selectedProviderDefinition?.keyField;

  const runLogs = useMemo(() => {
    if (!refine.partialLine) {
      return refine.logs;
    }
    return [...refine.logs, refine.partialLine];
  }, [refine.logs, refine.partialLine]);

  const setupStatusText = health?.healthy ? 'Backend connected' : 'Backend unavailable';
  const shouldFetchOpenAIModels = isAuthMode && selectedProvider === 'openai';
  const ideActions = useMemo(() => ideStatus?.actions ?? [], [ideStatus?.actions]);
  const finderAction = useMemo(() => ideActionFor(ideActions, 'finder'), [ideActions]);
  const xcodeAction = useMemo(() => ideActionFor(ideActions, 'xcode'), [ideActions]);
  const androidStudioAction = useMemo(() => ideActionFor(ideActions, 'android-studio'), [ideActions]);
  const vscodeAction = useMemo(() => ideActionFor(ideActions, 'vscode'), [ideActions]);

  useEffect(() => {
    if (allOptionValues.length === 0) {
      if (selectedModelOption !== '') {
        setSelectedModelOption('');
      }
      return;
    }

    if (!selectedModelOption || !allOptionValues.includes(selectedModelOption)) {
      setSelectedModelOption(allOptionValues[0] || '');
    }
  }, [allOptionValues, selectedModelOption]);

  const refreshSetup = async () => {
    setIsCheckingSetup(true);
    setHealthError(null);
    setAuthError(null);

    const [healthResult, authResult] = await Promise.allSettled([
      openCodeApi.getHealth(),
      openCodeApi.getAuthStatus(),
    ]);

    if (healthResult.status === 'fulfilled') {
      setHealth(healthResult.value);
    } else {
      setHealth(null);
      setHealthError(toErrorMessage(healthResult.reason, 'Failed to check backend health.'));
    }

    if (authResult.status === 'fulfilled') {
      setAuthStatus(authResult.value);
    } else {
      setAuthStatus(null);
      setAuthError(toErrorMessage(authResult.reason, 'Failed to fetch auth status.'));
    }

    setIsCheckingSetup(false);
  };

  const refreshIDEStatus = async (path: string) => {
    const trimmedPath = path.trim();
    if (!trimmedPath) {
      setIdeStatus(null);
      setIdeStatusError(null);
      return;
    }

    setIsLoadingIdeStatus(true);
    setIdeStatusError(null);

    try {
      const response = await openCodeApi.getProjectIDEStatus(trimmedPath);
      if (!response.success) {
        throw new Error(response.error || 'Failed to inspect project structure.');
      }
      setIdeStatus(response);
    } catch (error) {
      setIdeStatus(null);
      setIdeStatusError(toErrorMessage(error, 'Failed to inspect IDE project folders.'));
    } finally {
      setIsLoadingIdeStatus(false);
    }
  };

  useEffect(() => {
    void refreshSetup();
  }, []);

  useEffect(() => {
    const container = consoleRef.current;
    if (!container || !autoScrollEnabled) {
      return;
    }

    container.scrollTop = container.scrollHeight;
  }, [autoScrollEnabled, runLogs, refine.pendingQuestion, refine.pendingPermission]);

  useEffect(() => {
    setSimpleAnswerText('');
    setSimpleCustomAnswer('');
    setSimpleSelectedOptions([]);
    setNestedTextById({});
    setNestedCustomById({});
    setNestedSelectionsById({});
  }, [refine.pendingQuestion?.id]);

  useEffect(() => {
    let cancelled = false;

    if (!shouldFetchOpenAIModels) {
      setDynamicOpenAIModels([]);
      setOpenAIModelsInfo(null);
      setOpenAIModelsWarning(null);
      setIsLoadingOpenAIModels(false);
      return;
    }

    if (!isChatGPTConnected) {
      setDynamicOpenAIModels([]);
      setOpenAIModelsInfo(null);
      setOpenAIModelsWarning('Connect ChatGPT to load live OpenAI models.');
      setIsLoadingOpenAIModels(false);
      return;
    }

    setIsLoadingOpenAIModels(true);
    setOpenAIModelsInfo(null);
    setOpenAIModelsWarning(null);

    void openCodeApi
      .fetchOpenAIModels({
        projectPath: projectPath.trim() || undefined,
        openaiAuthMode: effectiveOpenAIAuthMode,
      })
      .then((response) => {
        if (cancelled) {
          return;
        }

        setDynamicOpenAIModels(response.models || []);
        setOpenAIModelsInfo(`Loaded ${response.models.length} OpenAI models from your local backend.`);

        if (response.debug?.usedFallbackAllowlist) {
          setOpenAIModelsWarning('Dynamic model catalog returned fallback defaults. You can still continue.');
        } else {
          setOpenAIModelsWarning(null);
        }
      })
      .catch((error) => {
        if (cancelled) {
          return;
        }

        setDynamicOpenAIModels([]);
        setOpenAIModelsInfo(null);
        setOpenAIModelsWarning(
          `Could not load dynamic OpenAI models. Showing fallback defaults. ${toErrorMessage(error, '')}`.trim(),
        );
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoadingOpenAIModels(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [
    effectiveOpenAIAuthMode,
    projectPath,
    isChatGPTConnected,
    shouldFetchOpenAIModels,
  ]);

  useEffect(() => {
    let cancelled = false;

    if (!isOpenCodeConfigMode) {
      setOpenCodeConfigProviders([]);
      setOpenCodeModelsInfo(null);
      setOpenCodeModelsWarning(null);
      setIsLoadingOpenCodeModels(false);
      setSelectedOpenCodeModel(OPENCODE_DEFAULT_MODEL_VALUE);
      return;
    }

    setIsLoadingOpenCodeModels(true);
    setOpenCodeModelsInfo(null);
    setOpenCodeModelsWarning(null);

    void openCodeApi
      .fetchOpenCodeAvailableModels({
        projectPath: projectPath.trim() || undefined,
        openaiAuthMode: effectiveOpenAIAuthMode,
        openaiKey: providerKeys.openaiKey.trim() || undefined,
        anthropicKey: providerKeys.anthropicKey.trim() || undefined,
        geminiKey: providerKeys.geminiKey.trim() || undefined,
        fireworksKey: providerKeys.fireworksKey.trim() || undefined,
        openrouterKey: providerKeys.openrouterKey.trim() || undefined,
        opencodeZenKey: providerKeys.opencodeZenKey.trim() || undefined,
        xaiKey: providerKeys.xaiKey.trim() || undefined,
      })
      .then((response) => {
        if (cancelled) {
          return;
        }

        const providers = response.providers || [];
        setOpenCodeConfigProviders(providers);
        const modelCount = providers.reduce((sum, provider) => sum + provider.models.length, 0);
        setOpenCodeModelsInfo(
          `Loaded ${providers.length} provider${providers.length === 1 ? '' : 's'} and ${modelCount} model${modelCount === 1 ? '' : 's'} from OpenCode config.`,
        );
        if (modelCount === 0) {
          setOpenCodeModelsWarning('OpenCode returned no models. You can still run with configured default model.');
        } else {
          setOpenCodeModelsWarning(null);
        }
      })
      .catch((error) => {
        if (cancelled) {
          return;
        }

        setOpenCodeConfigProviders([]);
        setOpenCodeModelsInfo(null);
        setOpenCodeModelsWarning(
          `Could not load OpenCode provider catalog. You can still run with configured default model. ${toErrorMessage(error, '')}`.trim(),
        );
      })
      .finally(() => {
        if (!cancelled) {
          setIsLoadingOpenCodeModels(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [
    effectiveOpenAIAuthMode,
    isOpenCodeConfigMode,
    projectPath,
    providerKeys.anthropicKey,
    providerKeys.fireworksKey,
    providerKeys.geminiKey,
    providerKeys.openaiKey,
    providerKeys.opencodeZenKey,
    providerKeys.openrouterKey,
    providerKeys.xaiKey,
  ]);

  useEffect(() => {
    if (!isOpenCodeConfigMode) {
      return;
    }

    if (selectedOpenCodeModel === OPENCODE_DEFAULT_MODEL_VALUE) {
      return;
    }

    const exists = openCodeConfigProviders.some((provider) =>
      provider.models.some((model) => `${provider.id}/${model.id}` === selectedOpenCodeModel),
    );

    if (!exists) {
      setSelectedOpenCodeModel(OPENCODE_DEFAULT_MODEL_VALUE);
    }
  }, [isOpenCodeConfigMode, openCodeConfigProviders, selectedOpenCodeModel]);

  const updateProviderKey = (field: keyof ProviderKeyState, value: string) => {
    setProviderKeys((previous) => ({
      ...previous,
      [field]: value,
    }));
  };

  const onConsoleScroll = () => {
    const element = consoleRef.current;
    if (!element) {
      return;
    }

    const distanceFromBottom = element.scrollHeight - element.scrollTop - element.clientHeight;
    setAutoScrollEnabled(distanceFromBottom < 24);
  };

  const openFolderPicker = async () => {
    setFolderPickerInfo(null);
    setFolderPickerWarning(null);
    setIsPickingFolder(true);

    try {
      const response = await openCodeApi.pickProjectFolder();

      if (response.success && response.path) {
        setProjectPath(response.path);
        setProjectEnvelope(null);
        setIdeStatus(null);
        setIdeStatusError(null);
        setIdeOpenInfo(null);
        setIdeOpenError(null);
        setProjectError(null);
        setFolderPickerInfo('Folder path selected locally. Click "Load Project".');
        return;
      }

      if (response.success && response.canceled) {
        setFolderPickerInfo('Folder selection canceled.');
        return;
      }

      const apiMessage = (response.error || '').trim();
      if (apiMessage) {
        if (apiMessage.toLowerCase().includes('only available on macos')) {
          setFolderPickerWarning('Native folder picker is macOS-only. Paste the project path manually.');
        } else {
          setFolderPickerWarning(`${apiMessage} Paste the project path manually if needed.`);
        }
        return;
      }
    } catch (error) {
      const message = toErrorMessage(error, 'unknown error').toLowerCase();
      if (message.includes('404') || message.includes('not found')) {
        setFolderPickerWarning(
          'Native folder picker API is unavailable. Restart the backend from latest code, then try again.',
        );
      } else {
        setFolderPickerWarning(
          `Native folder picker unavailable: ${toErrorMessage(error, 'unknown error')}. Paste the project path manually.`,
        );
      }
      return;
    } finally {
      setIsPickingFolder(false);
    }
  };

  const pickInstructionFiles = async () => {
    setInstructionPickerInfo(null);
    setInstructionPickerWarning(null);
    setIsPickingInstructionFiles(true);

    try {
      const response = await openCodeApi.pickInstructionFiles();
      if (response.success && response.canceled) {
        setInstructionPickerInfo('Attachment selection canceled.');
        return;
      }

      if (!response.success) {
        throw new Error(response.error || 'Failed to open local file picker.');
      }

      const pickedFiles = (response.files || []).filter((file) => file.path.trim().length > 0);
      if (pickedFiles.length === 0) {
        setInstructionPickerWarning('No files were selected.');
        return;
      }

      const oversizeFiles = pickedFiles.filter((file) => file.sizeBytes > MAX_INSTRUCTION_ATTACHMENT_BYTES);
      if (oversizeFiles.length > 0) {
        const first = oversizeFiles[0];
        if (first) {
          setInstructionPickerWarning(
            `"${first.name}" exceeds the 40MB limit (${formatAttachmentSize(first.sizeBytes)}). Remove it and try again.`,
          );
        }
      }

      setInstructionAttachments((previous) => {
        const merged = [...previous];
        for (const file of pickedFiles) {
          const exists = merged.some((entry) => entry.path === file.path);
          if (!exists) {
            merged.push(file);
          }
        }
        return merged;
      });

      const addedCount = pickedFiles.length;
      setInstructionPickerInfo(`Attached ${addedCount} local file${addedCount === 1 ? '' : 's'} for this refine run.`);
    } catch (error) {
      const message = toErrorMessage(error, 'unknown error').toLowerCase();
      if (message.includes('404') || message.includes('not found')) {
        setInstructionPickerWarning(
          'Instruction file picker API is unavailable. Restart the backend from latest code, then try again.',
        );
      } else {
        setInstructionPickerWarning(
          `Could not open local file picker: ${toErrorMessage(error, 'unknown error')}.`,
        );
      }
    } finally {
      setIsPickingInstructionFiles(false);
    }
  };

  const loadProject = async () => {
    const trimmedPath = projectPath.trim();
    if (!trimmedPath) {
      setProjectError('Enter a local Glowbom project path first.');
      return;
    }

    setProjectError(null);
    setProjectEnvelope(null);
    setIdeStatus(null);
    setIdeStatusError(null);
    setIdeOpenInfo(null);
    setIdeOpenError(null);
    setIsLoadingProject(true);

    try {
      const envelope = await openCodeApi.getProject(trimmedPath);
      if (!envelope.success || !envelope.project) {
        throw new Error(envelope.error || 'The provided path is not a valid Glowbom project.');
      }
      setProjectEnvelope(envelope);
      setProjectError(null);
      await refreshIDEStatus(trimmedPath);
    } catch (error) {
      setProjectError(toErrorMessage(error, 'Failed to load project.'));
    } finally {
      setIsLoadingProject(false);
    }
  };

  const openProjectInIDE = async (ide: OpenCodeIDE) => {
    const trimmedPath = projectPath.trim();
    if (!trimmedPath) {
      setIdeOpenError('Load a project first, then choose an IDE.');
      return;
    }

    setOpeningIDE(ide);
    setIdeOpenInfo(null);
    setIdeOpenError(null);

    try {
      const response = await openCodeApi.openProject({
        path: trimmedPath,
        ide,
      });
      if (!response.success) {
        throw new Error(response.error || 'Failed to open project.');
      }

      const action = ideActionFor(ideActions, ide);
      setIdeOpenInfo(`${action?.label || 'Open action'} triggered locally.`);
    } catch (error) {
      setIdeOpenError(toErrorMessage(error, 'Failed to open project locally.'));
    } finally {
      setOpeningIDE(null);
    }
  };

  const connectAuthProvider = async () => {
    setAuthConnectionInfo(null);
    setAuthConnectionError(null);

    if (authProvider !== 'chatgpt') {
      setAuthConnectionError('Selected auth provider is not supported yet.');
      return;
    }

    setIsUpdatingAuthConnection(true);
    let popupWindow: Window | null = null;
    try {
      popupWindow = window.open('', 'glowby-chatgpt-oauth', 'popup,width=560,height=760');
      if (!popupWindow) {
        throw new Error('Popup blocked. Allow popups for this site and try Connect again.');
      }

      const startResponse = await openCodeApi.startOpenAIOAuth({
        projectPath: projectPath.trim() || undefined,
      });
      if (!startResponse.success || !startResponse.authorizationURL) {
        throw new Error(startResponse.error || 'Failed to start ChatGPT login flow.');
      }

      popupWindow.location.href = startResponse.authorizationURL;
      setAuthConnectionInfo('ChatGPT login opened in a popup. Complete login to finish connection.');

      const timeoutAt = Date.now() + 5 * 60 * 1000;
      while (Date.now() < timeoutAt) {
        const oauthStatus = await openCodeApi.getOpenAIOAuthStatus(startResponse.state);
        if (oauthStatus.phase === 'succeeded') {
          setAuthStatus(oauthStatus.status);
          setAuthConnectionInfo('ChatGPT account connected. You can run Auth mode now.');
          try {
            popupWindow.close();
          } catch {}
          return;
        }
        if (oauthStatus.phase === 'failed') {
          throw new Error(oauthStatus.error || 'ChatGPT login failed.');
        }

        await new Promise<void>((resolve) => {
          window.setTimeout(resolve, 1000);
        });
      }

      throw new Error('Timed out waiting for ChatGPT login completion. Try Connect again.');
    } catch (error) {
      setAuthConnectionError(toErrorMessage(error, 'Failed to connect ChatGPT account.'));
      if (popupWindow && !popupWindow.closed) {
        try {
          popupWindow.close();
        } catch {}
      }
    } finally {
      setIsUpdatingAuthConnection(false);
    }
  };

  const disconnectAuthProvider = async () => {
    setAuthConnectionInfo(null);
    setAuthConnectionError(null);

    setIsUpdatingAuthConnection(true);
    try {
      const response = await openCodeApi.disconnectOpenAIAuth({
        projectPath: projectPath.trim() || undefined,
      });
      if (!response.success) {
        throw new Error(response.error || 'Failed to disconnect auth provider.');
      }

      setAuthStatus(response.status);
      setAuthConnectionInfo('ChatGPT account disconnected for this local OpenCode runtime.');
    } catch (error) {
      setAuthConnectionError(toErrorMessage(error, 'Failed to disconnect ChatGPT account.'));
    } finally {
      setIsUpdatingAuthConnection(false);
    }
  };

  const startRefine = () => {
    setFormError(null);

    if (!activeProject) {
      setFormError('Load a Glowbom project before starting refinement.');
      return;
    }

    const oversizedAttachment = instructionAttachments.find(
      (attachment) => attachment.sizeBytes > MAX_INSTRUCTION_ATTACHMENT_BYTES,
    );
    if (oversizedAttachment) {
      setFormError(
        `Attachment "${oversizedAttachment.name}" exceeds 40MB (${formatAttachmentSize(oversizedAttachment.sizeBytes)}). Remove it to continue.`,
      );
      return;
    }

    const instructionAttachmentPaths = instructionAttachments
      .map((attachment) => attachment.path.trim())
      .filter((value, index, values) => value.length > 0 && values.indexOf(value) === index);

    let modelValue = '';
    if (isOpenCodeConfigMode) {
      modelValue = selectedOpenCodeModel === OPENCODE_DEFAULT_MODEL_VALUE ? '' : selectedOpenCodeModel;
    } else {
      const customModelTrimmed = customModel.trim();
      const customModelProvider = inferProviderFromCustomModel(customModelTrimmed);
      const providerForValidation = customModelProvider || selectedProvider;

      if (!providerForValidation && !customModelTrimmed) {
        setFormError('Select a model before starting refinement.');
        return;
      }

      if (isAuthMode) {
        if (customModelProvider && customModelProvider !== 'openai') {
          setFormError('Auth provider mode currently supports OpenAI models only.');
          return;
        }
        if (!isChatGPTConnected) {
          setFormError('Connect ChatGPT first to run in Auth mode.');
          return;
        }
      }

      if (isApiKeyMode && providerForValidation) {
        const providerDef = providerDefinition(providerForValidation);
        if (providerDef?.requiresKey && providerDef.keyField) {
          const requiredValue = providerKeys[providerDef.keyField].trim();
          if (!requiredValue) {
            if (providerDef.id === 'openai') {
              setFormError('OpenAI model selected. Add your OpenAI API key before running refine.');
            } else {
              setFormError(`${providerDef.label} model selected. Add ${providerDef.keyLabel || 'the provider API key'}.`);
            }
            return;
          }
        }
      }

      modelValue = customModelTrimmed || resolvedSelection?.value || '';
      if (!modelValue) {
        setFormError('Model is required.');
        return;
      }
    }

    void refine.startRefine({
      projectPath: projectPath.trim(),
      instructions,
      persistCurrentInstructionsToHistory: true,
      instructionAttachmentPaths,
      model: modelValue || undefined,
      openaiAuthMode: effectiveOpenAIAuthMode,
      providerKeys,
    });
  };

  const submitQuestion = async () => {
    const question = refine.pendingQuestion;
    if (!question) {
      return;
    }

    if (question.questions.length === 0) {
      const custom = simpleCustomAnswer.trim();
      const freeText = simpleAnswerText.trim();
      const selected = simpleSelectedOptions;

      const response = custom ? [custom] : selected.length > 0 ? selected : freeText ? [freeText] : [];
      const answer = response.join(', ');

      await refine.submitQuestion({
        answer,
        answers: response.length > 0 ? [response] : undefined,
      });
      return;
    }

    const answers: string[][] = [];
    const answerByQuestionID: Record<string, string[]> = {};
    const summaryParts: string[] = [];

    for (const item of question.questions) {
      const custom = (nestedCustomById[item.id] || '').trim();
      const selected = nestedSelectionsById[item.id] || [];
      const freeText = (nestedTextById[item.id] || '').trim();

      const response = custom ? [custom] : selected.length > 0 ? selected : freeText ? [freeText] : [];
      answers.push(response);

      if (response.length > 0) {
        answerByQuestionID[item.id] = response;
      }

      summaryParts.push(`Q: ${item.prompt}\nA: ${response.join(', ')}`);
    }

    await refine.submitQuestion({
      answer: summaryParts.join('\n\n').trim(),
      answers,
      answerByQuestionID: Object.keys(answerByQuestionID).length > 0 ? answerByQuestionID : undefined,
    });
  };

  const renderQuestionItem = (item: OpenCodeQuestionItem) => {
    const multi = isMultiSelect(item.prompt, item.inputType);
    const selected = nestedSelectionsById[item.id] || [];

    return (
      <div className="question-item" key={item.id}>
        <p className="question-item-title">{item.prompt}</p>

        {item.options.length > 0 ? (
          <div className="option-list">
            {item.options.map((option) => {
              const isSelected = selected.includes(option.label);

              return (
                <button
                  className={`option-button ${isSelected ? 'selected' : ''}`}
                  key={option.id || option.label}
                  onClick={() => {
                    setNestedSelectionsById((previous) => ({
                      ...previous,
                      [item.id]: toggleSelection(previous[item.id] || [], option.label, multi),
                    }));
                  }}
                  type="button"
                >
                  <span className="option-button-label">{option.label}</span>
                  {option.description ? <span className="option-button-description">{option.description}</span> : null}
                </button>
              );
            })}
          </div>
        ) : null}

        <textarea
          className="input"
          onChange={(event) => {
            setNestedTextById((previous) => ({
              ...previous,
              [item.id]: event.target.value,
            }));
          }}
          placeholder="Answer (optional)"
          rows={2}
          value={nestedTextById[item.id] || ''}
        />

        <input
          className="input"
          onChange={(event) => {
            setNestedCustomById((previous) => ({
              ...previous,
              [item.id]: event.target.value,
            }));
          }}
          placeholder="Custom response override (optional)"
          type="text"
          value={nestedCustomById[item.id] || ''}
        />
      </div>
    );
  };

  const renderConsoleLine = (line: string, index: number, isPartial: boolean) => {
    const normalized = line || ' ';
    const heading = parseConsoleHeading(normalized);
    const sharedClassName = `console-line${isPartial ? ' partial' : ''}`;

    if (heading) {
      return (
        <p className={`${sharedClassName} console-heading heading-${heading.level}`} key={`${index}-${line}`}>
          {heading.text}
        </p>
      );
    }

    if (lineLooksLikeMarkdown(normalized)) {
      const markdownHTML = parseConsoleMarkdown(normalized);
      if (markdownHTML) {
        return (
          <p
            className={`${sharedClassName} console-markdown`}
            dangerouslySetInnerHTML={{ __html: markdownHTML || '&nbsp;' }}
            key={`${index}-${line}`}
          />
        );
      }
    }

    return (
      <pre className={sharedClassName} key={`${index}-${line}`}>
        {normalized}
      </pre>
    );
  };

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="topbar-inner">
          <a className="topbar-logo" href="https://glowbom.com" rel="noreferrer" target="_blank">
            <img alt="Glowbom" src="/logo-svg.svg" />
          </a>
          <nav className="topbar-links">
            <a href="https://glowbom.com/glowby/" rel="noreferrer" target="_blank">
              Glowby
            </a>
            <a href="https://glowbom.com/desktop/" rel="noreferrer" target="_blank">
              Desktop
            </a>
            <a href="https://glowbom.com/terms.html" rel="noreferrer" target="_blank">
              Terms
            </a>
            <a href="https://glowbom.com/pricing/" rel="noreferrer" target="_blank">
              Pricing
            </a>
            <a href="https://glowbom.com/docs/" rel="noreferrer" target="_blank">
              Docs
            </a>
            <a href="https://glowbom.com/#case_studies" rel="noreferrer" target="_blank">
              Apps
            </a>
          </nav>
          <a className="button topbar-cta" href="https://glowbom.com/draw" rel="noreferrer" target="_blank">
            Get Started for Free
          </a>
        </div>
      </header>

      <main className="page">
        <header className="brand-header">
          <div className="brand-copy">
            <h1>Glowby</h1>
            <p>Choose a project folder and let Glowby finish the engineering work locally.</p>
          </div>
        </header>

        <section className="card">
        <div className="card-title-row">
          <h2>1. Setup</h2>
          <button
            className="button secondary"
            disabled={isCheckingSetup || refine.isRunning}
            onClick={() => void refreshSetup()}
          >
            {isCheckingSetup ? 'Checking...' : 'Refresh'}
          </button>
        </div>

        <div className="status-grid">
          <div className="status-tile">
            <span className="status-label">Backend</span>
            <strong className={health?.healthy ? 'ok' : 'warn'}>{setupStatusText}</strong>
            {health?.server ? <span className="meta">{health.server}</span> : null}
            {health?.hint ? <span className="meta">{health.hint}</span> : null}
            {healthError ? <span className="error-inline">{healthError}</span> : null}
          </div>

          <div className="status-tile">
            <span className="status-label">Auth Diagnostics</span>
            <strong>{authStatus?.serverRunning ? 'Agent server running' : 'Agent server stopped'}</strong>
            <span className="meta">Credential type: {formatCredentialType(authStatus?.openaiCredentialType)}</span>
            <span className="meta">Cached auth mode: {formatAuthMode(authStatus?.cachedGlowbomAuthMode)}</span>
            {authError ? <span className="error-inline">{authError}</span> : null}
          </div>
        </div>

        <label className="field-label" htmlFor="projectPath">
          Glowbom project path
        </label>
        <div className="row compact">
          <input
            className="input"
            id="projectPath"
            onChange={(event) => {
              setProjectPath(event.target.value);
              setProjectEnvelope(null);
              setIdeStatus(null);
              setIdeStatusError(null);
              setIdeOpenInfo(null);
              setIdeOpenError(null);
            }}
            placeholder="/absolute/path/to/project"
            value={projectPath}
          />
          <button className="button secondary" disabled={refine.isRunning || isPickingFolder} onClick={() => void openFolderPicker()}>
            {isPickingFolder ? 'Choosing...' : 'Choose Folder'}
          </button>
          <button className="button" disabled={isLoadingProject || refine.isRunning} onClick={() => void loadProject()}>
            {isLoadingProject ? 'Loading...' : 'Load Project'}
          </button>
        </div>
        <p className="meta">Choose Folder opens a native local picker and sets the path field. No uploads.</p>

        {folderPickerInfo ? <p className="meta ok">{folderPickerInfo}</p> : null}
        {folderPickerWarning ? <p className="meta warn">{folderPickerWarning}</p> : null}
        {projectError ? <p className="error-inline">{projectError}</p> : null}

        {activeProject ? (
          <div className="project-summary">
            <div className="project-summary-header">
              <strong>{activeProject.name}</strong>
              <span>v{activeProject.version}</span>
            </div>
            <div className="chip-list">
              {targetRows.map((target) => (
                <span className="chip" key={target.id}>
                  {target.id} · {target.stack} · {target.outputDir}
                </span>
              ))}
            </div>

            <div className="project-ide-tools">
              <div className="card-title-row">
                <strong>Open Project</strong>
                <button
                  className="button secondary tiny"
                  disabled={isLoadingIdeStatus}
                  onClick={() => void refreshIDEStatus(projectPath)}
                >
                  {isLoadingIdeStatus ? 'Scanning...' : 'Refresh IDE Scan'}
                </button>
              </div>

              <div className="row">
                <button
                  className="button secondary"
                  disabled={!finderAction?.available || openingIDE !== null}
                  onClick={() => {
                    void openProjectInIDE('finder');
                  }}
                >
                  {openingIDE === 'finder' ? 'Opening...' : 'Open Folder'}
                </button>
                <button
                  className="button secondary"
                  disabled={!xcodeAction?.available || openingIDE !== null}
                  onClick={() => {
                    void openProjectInIDE('xcode');
                  }}
                >
                  {openingIDE === 'xcode' ? 'Opening...' : 'Open in Xcode'}
                </button>
                <button
                  className="button secondary"
                  disabled={!androidStudioAction?.available || openingIDE !== null}
                  onClick={() => {
                    void openProjectInIDE('android-studio');
                  }}
                >
                  {openingIDE === 'android-studio' ? 'Opening...' : 'Open in Android Studio'}
                </button>
                <button
                  className="button secondary"
                  disabled={!vscodeAction?.available || openingIDE !== null}
                  onClick={() => {
                    void openProjectInIDE('vscode');
                  }}
                >
                  {openingIDE === 'vscode' ? 'Opening...' : 'Open in VS Code'}
                </button>
              </div>

              {ideActions.length > 0 ? (
                <div className="ide-diagnostics">
                  {ideActions.map((action) => (
                    <p className={`meta ${action.available ? 'ok' : 'warn'}`} key={action.ide}>
                      {action.label}: {action.reason || (action.available ? 'Ready' : 'Not ready')}
                    </p>
                  ))}
                </div>
              ) : null}

              {ideStatusError ? <p className="error-inline">{ideStatusError}</p> : null}
              {ideOpenInfo ? <p className="meta ok">{ideOpenInfo}</p> : null}
              {ideOpenError ? <p className="error-inline">{ideOpenError}</p> : null}
            </div>
          </div>
        ) : null}
      </section>

      <section className="card">
        <h2>2. Refine Config</h2>

        <div className="field-grid">
          <div>
            <label className="field-label" htmlFor="credentialMode">
              Connection mode
            </label>
            <select
              className="input"
              disabled={refine.isRunning}
              id="credentialMode"
              onChange={(event) => setCredentialMode(event.target.value as CredentialMode)}
              value={credentialMode}
            >
              <option value="auth">Auth Providers</option>
              <option value="api-key">API Keys (provider keys)</option>
              <option value="opencode-config">Use OpenCode Config</option>
            </select>
          </div>

          {isAuthMode ? (
            <div>
              <label className="field-label" htmlFor="authProvider">
                Auth provider
              </label>
              <select
                className="input"
                disabled={refine.isRunning || isUpdatingAuthConnection}
                id="authProvider"
                onChange={(event) => {
                  setAuthProvider(event.target.value as AuthProviderID);
                  setAuthConnectionInfo(null);
                  setAuthConnectionError(null);
                }}
                value={authProvider}
              >
                <option value="chatgpt">ChatGPT (recommended)</option>
              </select>
            </div>
          ) : null}
        </div>

        {isAuthMode ? (
          <div className="chatgpt-callout">
            <h3>Use your ChatGPT account</h3>
            <p>Auth providers are account-level connections. ChatGPT is available now and is recommended.</p>
            <div className={`auth-connection-card ${authProviderConnected ? 'connected' : 'disconnected'}`}>
              <span className="status-label">Connection status</span>
              <strong className={authProviderConnected ? 'ok' : 'warn'}>
                {authProviderConnected ? 'Connected' : 'Not connected'}
              </strong>
              <span className="meta">Provider: ChatGPT</span>
              <span className="meta">Credential type: {formatCredentialType(authStatus?.openaiCredentialType)}</span>
              <span className="meta">Cached auth mode: {formatAuthMode(authStatus?.cachedGlowbomAuthMode)}</span>
              <div className="row">
                {authProviderConnected ? (
                  <button
                    className="button secondary"
                    disabled={refine.isRunning || isUpdatingAuthConnection}
                    onClick={() => {
                      void disconnectAuthProvider();
                    }}
                    type="button"
                  >
                    {isUpdatingAuthConnection ? 'Disconnecting...' : 'Disconnect'}
                  </button>
                ) : (
                  <button
                    className="button"
                    disabled={refine.isRunning || isUpdatingAuthConnection}
                    onClick={() => {
                      void connectAuthProvider();
                    }}
                    type="button"
                  >
                    {isUpdatingAuthConnection ? 'Connecting...' : 'Connect'}
                  </button>
                )}
              </div>
            </div>
            <ol>
              <li>Select <strong>Auth Providers</strong> mode.</li>
              <li>Choose <strong>ChatGPT</strong> provider.</li>
              <li>Click <strong>Connect</strong> to use your local ChatGPT/Codex login.</li>
            </ol>
            {authConnectionInfo ? <p className="meta ok">{authConnectionInfo}</p> : null}
            {authConnectionError ? <p className="error-inline">{authConnectionError}</p> : null}
          </div>
        ) : null}

        {isApiKeyMode ? (
          <div className="chatgpt-callout">
            <h3>Use provider API keys</h3>
            <p>Select any supported provider/model and add the matching provider key.</p>
          </div>
        ) : null}

        {isOpenCodeConfigMode ? (
          <div className="chatgpt-callout">
            <h3>Use whatever OpenCode already has</h3>
            <p>No key input required here. Glowby OSS will use current OpenCode configuration and available providers/models.</p>
          </div>
        ) : null}

        <div className="field-grid">
          <div>
            <label className="field-label" htmlFor="modelPreset">
              Agent model
            </label>
            {isOpenCodeConfigMode ? (
              <select
                className="input"
                disabled={refine.isRunning || isLoadingOpenCodeModels}
                id="modelPreset"
                onChange={(event) => setSelectedOpenCodeModel(event.target.value)}
                value={selectedOpenCodeModel}
              >
                <option value={OPENCODE_DEFAULT_MODEL_VALUE}>Use OpenCode default model (configured)</option>
                {openCodeConfigProviders.map((provider) => (
                  <optgroup key={provider.id} label={provider.displayName || provider.id}>
                    {provider.models.map((model) => (
                      <option key={`${provider.id}-${model.id}`} value={`${provider.id}/${model.id}`}>
                        {model.displayName || model.id}
                      </option>
                    ))}
                  </optgroup>
                ))}
              </select>
            ) : (
              <select
                className="input"
                disabled={refine.isRunning || allOptionValues.length === 0}
                id="modelPreset"
                onChange={(event) => setSelectedModelOption(event.target.value)}
                value={selectedModelOption}
              >
                {visibleModelGroups.map((group) => (
                  <optgroup key={group.provider.id} label={group.provider.label}>
                    {group.models.map((model) => {
                      const value = encodeModelOptionValue(group.provider.id, model.id);
                      return (
                        <option key={`${group.provider.id}-${model.id}`} value={value}>
                          {model.label}
                        </option>
                      );
                    })}
                  </optgroup>
                ))}
              </select>
            )}

            {!isOpenCodeConfigMode && resolvedSelection ? <p className="meta">Selected: {resolvedSelection.value}</p> : null}
            {isLoadingOpenAIModels ? <p className="meta">Loading OpenAI model catalog...</p> : null}
            {openAIModelsInfo ? <p className="meta ok">{openAIModelsInfo}</p> : null}
            {openAIModelsWarning ? <p className="meta warn">{openAIModelsWarning}</p> : null}
            {isLoadingOpenCodeModels ? <p className="meta">Loading OpenCode provider catalog...</p> : null}
            {openCodeModelsInfo ? <p className="meta ok">{openCodeModelsInfo}</p> : null}
            {openCodeModelsWarning ? <p className="meta warn">{openCodeModelsWarning}</p> : null}
          </div>

          {!isOpenCodeConfigMode ? (
            <div>
              <label className="field-label" htmlFor="customModel">
                Custom model override (optional)
              </label>
              <input
                className="input"
                disabled={refine.isRunning}
                id="customModel"
                onChange={(event) => setCustomModel(event.target.value)}
                placeholder="provider/model"
                value={customModel}
              />
              <p className="meta">If set, this overrides the dropdown selection.</p>
            </div>
          ) : null}
        </div>

        {isAuthMode ? (
          <p className="meta">
            Token fields are hidden in Auth mode. Connect starts ChatGPT OAuth and links your local OpenCode runtime.
          </p>
        ) : null}

        {isApiKeyMode ? (
          <div className="field-grid">
            <div>
              <label className="field-label" htmlFor="openaiKey">
                OpenAI API key
              </label>
              <input
                className="input"
                disabled={refine.isRunning}
                id="openaiKey"
                onChange={(event) => updateProviderKey('openaiKey', event.target.value)}
                placeholder="sk-..."
                type="password"
                value={providerKeys.openaiKey}
              />
            </div>
          </div>
        ) : null}

        {isApiKeyMode ? (
          <details className="provider-keys">
            <summary>Provider Keys</summary>
            <p className="meta">Add keys for non-OpenAI providers. The selected model's provider key is required.</p>
            <div className="field-grid provider-keys-grid">
              {MODEL_PROVIDERS.filter((provider) => provider.keyField && provider.id !== 'openai').map((provider) => {
                const keyField = provider.keyField!;
                const isActive = provider.id === selectedProvider;

                return (
                  <label className={`provider-key-item ${isActive ? 'active' : ''}`} key={provider.id}>
                    <span className="field-label">{provider.keyLabel || provider.label}</span>
                    <input
                      className="input"
                      disabled={refine.isRunning}
                      onChange={(event) => updateProviderKey(keyField, event.target.value)}
                      placeholder={`${provider.label} key`}
                      type="password"
                      value={providerKeys[keyField]}
                    />
                  </label>
                );
              })}
            </div>
          </details>
        ) : null}

        {isApiKeyMode && selectedProviderKeyField && selectedProviderDefinition?.requiresKey ? (
          <p className="meta">
            Selected provider: {selectedProviderDefinition.label}.
            {providerKeys[selectedProviderKeyField].trim()
              ? ' Credential detected.'
              : ` Missing ${selectedProviderDefinition.keyLabel || 'provider key'}.`}
          </p>
        ) : null}

        <label className="field-label" htmlFor="instructions">
          Refine instructions
        </label>
        <textarea
          className="input"
          disabled={refine.isRunning}
          id="instructions"
          onChange={(event) => setInstructions(event.target.value)}
          rows={5}
          value={instructions}
        />

        <div className="instruction-attachments">
          <div className="card-title-row">
            <strong>Instruction attachments</strong>
            <div className="row">
              <button
                className="button secondary tiny"
                disabled={refine.isRunning || isPickingInstructionFiles}
                onClick={() => {
                  void pickInstructionFiles();
                }}
                type="button"
              >
                {isPickingInstructionFiles ? 'Choosing...' : 'Attach Files'}
              </button>
              <button
                className="button secondary tiny"
                disabled={refine.isRunning || instructionAttachments.length === 0}
                onClick={() => {
                  setInstructionAttachments([]);
                  setInstructionPickerInfo(null);
                  setInstructionPickerWarning(null);
                }}
                type="button"
              >
                Clear
              </button>
            </div>
          </div>
          <p className="meta">Local-only picker. No uploads. Max 40MB per file.</p>

          {instructionPickerInfo ? <p className="meta ok">{instructionPickerInfo}</p> : null}
          {instructionPickerWarning ? <p className="meta warn">{instructionPickerWarning}</p> : null}

          {instructionAttachments.length > 0 ? (
            <ul className="attachment-list">
              {instructionAttachments.map((attachment) => {
                const oversize = attachment.sizeBytes > MAX_INSTRUCTION_ATTACHMENT_BYTES;
                return (
                  <li className={`attachment-item ${oversize ? 'oversize' : ''}`} key={attachment.path}>
                    <div className="attachment-item-copy">
                      <span className="attachment-name">{attachment.name}</span>
                      <span className="attachment-meta">
                        {formatAttachmentSize(attachment.sizeBytes)}
                        {attachment.mimeType ? ` · ${attachment.mimeType}` : ''}
                      </span>
                    </div>
                    <button
                      className="button secondary tiny"
                      disabled={refine.isRunning}
                      onClick={() => {
                        setInstructionAttachments((previous) =>
                          previous.filter((item) => item.path !== attachment.path),
                        );
                      }}
                      type="button"
                    >
                      Remove
                    </button>
                  </li>
                );
              })}
            </ul>
          ) : (
            <p className="meta">No files attached yet.</p>
          )}
        </div>

        {formError ? <p className="error-inline">{formError}</p> : null}
      </section>

      <section className="card">
        <div className="card-title-row">
          <h2>3. Run</h2>
          <span className={`run-status status-${refine.status}`}>{RUN_STATUS_LABEL[refine.status]}</span>
        </div>

        <div className="row">
          <button
            className="button"
            disabled={refine.isRunning || refine.isSubmittingInput || !activeProject}
            onClick={startRefine}
          >
            Refine with Agent
          </button>
          <button className="button danger" disabled={!refine.isRunning} onClick={refine.stopRun}>
            Stop
          </button>
        </div>
        {refine.hasSession && !refine.isRunning ? (
          <label className="toggle-label">
            <input
              type="checkbox"
              checked={refine.continueSession}
              onChange={(e) => refine.setContinueSession(e.target.checked)}
            />
            Continue previous session
          </label>
        ) : null}

        {refine.summary ? <p className="summary-text">{refine.summary}</p> : null}
        {refine.error ? <p className="error-inline">{refine.error}</p> : null}

        {refine.changedFiles.length > 0 ? (
          <details className="changed-files">
            <summary>Changed files ({refine.changedFiles.length})</summary>
            <ul>
              {refine.changedFiles.slice(0, 200).map((filePath) => (
                <li key={filePath}>{filePath}</li>
              ))}
            </ul>
          </details>
        ) : null}
      </section>

      <section className="card">
        <div className="card-title-row">
          <h2>4. Console</h2>
          <label className="checkbox-row">
            <input
              checked={autoScrollEnabled}
              onChange={(event) => setAutoScrollEnabled(event.target.checked)}
              type="checkbox"
            />
            auto-scroll
          </label>
        </div>

        <div className="console" onScroll={onConsoleScroll} ref={consoleRef}>
          {runLogs.length === 0 ? (
            <p className="placeholder">Ready. Load a project and start refinement.</p>
          ) : (
            runLogs.map((line, index) => renderConsoleLine(line, index, index === runLogs.length - 1 && !!refine.partialLine))
          )}
        </div>

        {refine.pendingQuestion ? (
          <div className="input-panel">
            <h3>Agent needs input</h3>
            <p>{refine.pendingQuestion.prompt}</p>

            {refine.pendingQuestion.questions.length > 0 ? (
              <div className="question-list">{refine.pendingQuestion.questions.map(renderQuestionItem)}</div>
            ) : (
              <div className="question-item">
                {refine.pendingQuestion.options.length > 0 ? (
                  <div className="option-list">
                    {refine.pendingQuestion.options.map((option) => {
                      const multi = isMultiSelect(refine.pendingQuestion?.prompt || '');
                      const selected = simpleSelectedOptions.includes(option.label);

                      return (
                        <button
                          className={`option-button ${selected ? 'selected' : ''}`}
                          key={option.id || option.label}
                          onClick={() => {
                            setSimpleSelectedOptions((previous) =>
                              toggleSelection(previous, option.label, multi),
                            );
                          }}
                          type="button"
                        >
                          <span className="option-button-label">{option.label}</span>
                          {option.description ? (
                            <span className="option-button-description">{option.description}</span>
                          ) : null}
                        </button>
                      );
                    })}
                  </div>
                ) : null}

                <textarea
                  className="input"
                  onChange={(event) => setSimpleAnswerText(event.target.value)}
                  placeholder="Your answer"
                  rows={3}
                  value={simpleAnswerText}
                />

                <input
                  className="input"
                  onChange={(event) => setSimpleCustomAnswer(event.target.value)}
                  placeholder="Custom response override (optional)"
                  type="text"
                  value={simpleCustomAnswer}
                />
              </div>
            )}

            <div className="row">
              <button
                className="button"
                disabled={refine.isSubmittingInput}
                onClick={() => {
                  void submitQuestion();
                }}
              >
                {refine.isSubmittingInput ? 'Sending...' : 'Send'}
              </button>
              <button
                className="button secondary"
                disabled={refine.isSubmittingInput}
                onClick={() => {
                  void refine.submitQuestion({ answer: '' });
                }}
              >
                Dismiss
              </button>
            </div>
          </div>
        ) : null}

        {refine.pendingPermission ? (
          <div className="input-panel">
            <h3>Permission requested</h3>
            <p>{refine.pendingPermission.title}</p>
            {refine.pendingPermission.message ? <p className="meta-text">{refine.pendingPermission.message}</p> : null}
            {refine.pendingPermission.pattern ? (
              <p className="meta-text">Pattern: {refine.pendingPermission.pattern}</p>
            ) : null}

            <div className="row">
              <button
                className="button"
                disabled={refine.isSubmittingInput}
                onClick={() => {
                  void refine.respondToPermission('once');
                }}
              >
                Allow once
              </button>
              <button
                className="button secondary"
                disabled={refine.isSubmittingInput}
                onClick={() => {
                  void refine.respondToPermission('always');
                }}
              >
                Always allow
              </button>
              <button
                className="button danger"
                disabled={refine.isSubmittingInput}
                onClick={() => {
                  void refine.respondToPermission('reject');
                }}
              >
                Deny
              </button>
            </div>
          </div>
        ) : null}
      </section>
      </main>
    </div>
  );
}
