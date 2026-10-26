<template>
  <div class="modal-card conflict-modal">
    <header class="modal-card-head">
      <p class="modal-card-title">
        <b-icon icon="exclamation-triangle" size="is-small" class="has-text-warning mr-2" />
        {{ lang('Items already exist') }}
      </p>
    </header>
    <section class="modal-card-body">
      <p class="mb-3">
        {{ lang('The following items already exist in the destination:') }}
      </p>
      <div class="conflict-list">
        <div v-for="(item, index) in conflicts" :key="index" class="conflict-item">
          <b-icon :icon="item.type == 'dir' ? 'folder' : 'file'" size="is-small" />
          <span class="conflict-name">{{ item.name }}</span>
        </div>
      </div>
      <p class="is-size-7 has-text-grey mt-3">
        {{ lang('Choose "Replace" to overwrite files and combine folders, or "Rename" to keep both.') }}
      </p>
    </section>
    <footer class="modal-card-foot is-justify-between">
      <div>
        <button class="button is-danger mr-2" type="button" @click="resolve('replace')">
          {{ lang('Replace') }}
        </button>
        <button class="button is-info" type="button" @click="resolve('rename')">
          {{ lang('Rename') }}
        </button>
      </div>
      <button class="button" type="button" @click="$parent.close()">
        {{ lang('Cancel') }}
      </button>
    </footer>
  </div>
</template>

<script>
export default {
  name: 'ConflictModal',
  props: {
    conflicts: {
      type: Array,
      default: () => [],
    },
  },
  methods: {
    resolve(action) {
      this.$emit('resolve', action)
      this.$parent.close()
    },
  },
}
</script>

<style scoped>
.conflict-list {
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid #e1e4e8;
  border-radius: 4px;
  padding: 8px 12px;
  background-color: #fafbfc;
}
.conflict-item {
  display: flex;
  align-items: center;
  padding: 4px 0;
}
.conflict-item .icon {
  margin-right: 8px;
}
.conflict-name {
  font-weight: 500;
  word-break: break-all;
}
.is-justify-between {
  display: flex;
  justify-content: space-between;
}
.mr-2 {
  margin-right: 0.5rem;
}
.mt-3 {
  margin-top: 0.75rem;
}
.mb-3 {
  margin-bottom: 0.75rem;
}
</style>
