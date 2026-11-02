<template>
  <div class="card-content">
    <p>Download a file from a public HTTP or HTTPS URL into the currently open folder.</p>
    <label for="url-download-input">Download URL</label>
    <input id="url-download-input" class="input" type="url" v-model.trim="downloadURL" placeholder="https://example.com/file.zip" :disabled="downloading" @keyup.enter="submit" />
    <p v-if="error" class="error-message" role="alert">{{ error }}</p>
    <div v-if="downloading" class="loading-content">
      <LoadingSpinner size="small" />
      <p class="loading-text">Downloading on the server…</p>
      <button type="button" class="button button--flat button--grey" @click="cancel">Cancel</button>
    </div>
  </div>
  <div class="card-actions">
    <button type="button" class="button button--flat button--grey" :disabled="downloading" @click="close">Cancel</button>
    <button type="button" class="button button--flat" :disabled="downloading || !downloadURL" @click="submit">Download</button>
  </div>
</template>
<script>
import { fetchURL } from "@/api/utils.ts";
import { getApiPath } from "@/utils/url.js";
import { mutations, state } from "@/store";
import LoadingSpinner from "@/components/LoadingSpinner.vue";

export default {
  name: "URLDownload",
  components: { LoadingSpinner },
  props: { promptId: { type: [String, Number], default: null }, targetPath: { type: String, required: true }, targetSource: { type: String, required: true } },
  data() { return { downloadURL: "", downloading: false, error: "", controller: null }; },
  methods: {
    close() { mutations.closeTopPrompt(); },
    cancel() { this.controller?.abort(); },
    async submit() {
      if (!this.downloadURL || this.downloading) return;
      this.error = ""; this.downloading = true; this.controller = new AbortController();
      try {
        await fetchURL(getApiPath("resources/url-download"), {
          method: "POST", headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ url: this.downloadURL, path: this.targetPath, source: this.targetSource }),
          signal: this.controller.signal,
        });
        mutations.setReload(true);
        mutations.closeTopPrompt();
      } catch (err) {
        if (err?.name !== "AbortError") this.error = err?.message || "Download failed";
      } finally { this.downloading = false; this.controller = null; }
    },
  },
};
</script>
