<template>
  <div class="modal-card">
    <header class="modal-card-head">
      <p class="modal-card-title">
        {{ lang('Select Folder') }}
      </p>
    </header>
    <section class="modal-card-body">
      <div class="tree">
        <ul class="tree-list">
          <TreeNode :node="$store.state.tree" @selected="$emit('selected', $event) && $parent.close()" />
        </ul>
      </div>
    </section>
    <footer class="modal-card-foot">
      <button class="button" type="button" @click="$parent.close()">
        {{ lang('Close') }}
      </button>
    </footer>
  </div>
</template>

<script>
import TreeNode from './TreeNode'

export default {
  name: 'Tree',
  components: { TreeNode },
}
</script>

<style>
.tree {
  min-height: 450px
}

.tree-list ul li {
  padding-left: 20px;
  margin: 6px 0;
}
</style>
