<template>
  <div>
    <b-select :value="perpage" size="is-small" @input="$emit('selected', $event)">
      <option value="">
        {{ lang('No pagination') }}
      </option>
      <option value="5">
        {{ lang('Per page', 5) }}
      </option>
      <option value="10">
        {{ lang('Per page', 10) }}
      </option>
      <option value="15">
        {{ lang('Per page', 15) }}
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

