import type { DirectiveBinding } from 'vue';

export const draggable = {
  beforeMount(el: HTMLElement, binding: DirectiveBinding) {
    let startX = 0;
    let startWidth = 0;

    const mouseMoveHandler = (event: MouseEvent) => {
      const newWidth = startWidth + (event.clientX - startX);

      // Ensure the width stays within a reasonable range
      if (newWidth >= 300 && newWidth <= 600) {
        el.style.width = `${newWidth}px`;

        binding.value.width = newWidth;
      }
    };

    const mouseUpHandler = () => {
      document.removeEventListener('mousemove', mouseMoveHandler);
      document.removeEventListener('mouseup', mouseUpHandler);

      if (binding.value.onDragEnd && typeof binding.value.onDragEnd === 'function') {
        binding.value.onDragEnd(el, binding.value.width);
      }
    };

    const mouseDownHandler = (event: MouseEvent) => {
      const rect = el.getBoundingClientRect();
      // Only trigger when dragging within 10px of the right edge
      if (event.clientX >= rect.right - 10 && event.clientX <= rect.right) {
        startX = event.clientX;
        startWidth = el.offsetWidth;

        document.addEventListener('mousemove', mouseMoveHandler);
        document.addEventListener('mouseup', mouseUpHandler);
      }
    };

    el.addEventListener('mousedown', mouseDownHandler);
  },
};
