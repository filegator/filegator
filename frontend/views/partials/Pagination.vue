<template>
  <div>
    <b-select :value="perpage" size="is-small" @input="$emit('selected', $event)">
      <option
        v-for="page in this.$store.state.config.pagination"
        :key="page"
        :value="page"
      >
        {{ page === '' ? lang('No pagination') : lang('Per page', page) }}
      </option>
    </b-select>
  </div>
</template>

<script>

export default {
  name: 'Pagination',
  props: [ 'perpage' ]
}
</script>

