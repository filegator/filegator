<template>
  <div>
      <b-select :value="perpage" @input="$emit('selected', $event)" size="is-small">
        <option value="">{{ lang('No pagination') }}</option>
        <option value="5">{{ lang('Per page', 5) }}</option>
        <option value="10">{{ lang('Per page', 10) }}</option>
        <option value="15">{{ lang('Per page', 15) }}</option>
      </b-select>
  </div>
</template>

<script>

export default {
  name: 'Pagination',
  props: [ 'perpage' ]
}
</script>

