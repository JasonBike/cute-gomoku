<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";
import type { Coordinate, Move } from "@/types/game";

const props = defineProps<{
  board: number[][];
  moves: Move[];
  winningLine?: Coordinate[];
  disabled?: boolean;
}>();

const emit = defineEmits<{
  move: [row: number, column: number];
  occupied: [];
}>();

const canvas = ref<HTMLCanvasElement | null>(null);
const size = 15;
const internalSize = 720;
const padding = 42;
const boardLength = internalSize - padding * 2;
const cellSize = boardLength / (size - 1);

function draw() {
  const element = canvas.value;
  const context = element?.getContext("2d");
  if (!element || !context) return;

  context.clearRect(0, 0, internalSize, internalSize);
  const paper = context.createLinearGradient(0, 0, internalSize, internalSize);
  paper.addColorStop(0, "#f3ca89");
  paper.addColorStop(1, "#e6b26a");
  context.fillStyle = paper;
  context.fillRect(0, 0, internalSize, internalSize);

  context.strokeStyle = "rgba(82, 56, 35, 0.78)";
  context.lineWidth = 2;
  for (let index = 0; index < size; index += 1) {
    const position = padding + cellSize * index;
    context.beginPath();
    context.moveTo(padding, position);
    context.lineTo(internalSize - padding, position);
    context.stroke();
    context.beginPath();
    context.moveTo(position, padding);
    context.lineTo(position, internalSize - padding);
    context.stroke();
  }

  for (const row of [3, 7, 11]) {
    for (const column of [3, 7, 11]) {
      context.beginPath();
      context.arc(
        padding + column * cellSize,
        padding + row * cellSize,
        row === 7 && column === 7 ? 6 : 4.5,
        0,
        Math.PI * 2,
      );
      context.fillStyle = "rgba(77, 51, 31, 0.82)";
      context.fill();
    }
  }

  props.moves.forEach((move, index) => {
    drawStone(context, move, index === props.moves.length - 1);
  });

  if (props.winningLine?.length === 2) {
    const [start, end] = props.winningLine;
    context.beginPath();
    context.moveTo(padding + start.column * cellSize, padding + start.row * cellSize);
    context.lineTo(padding + end.column * cellSize, padding + end.row * cellSize);
    context.strokeStyle = "#ff795f";
    context.lineWidth = 9;
    context.lineCap = "round";
    context.globalAlpha = 0.82;
    context.stroke();
    context.globalAlpha = 1;
  }
}

function drawStone(context: CanvasRenderingContext2D, move: Move, last: boolean) {
  const x = padding + move.column * cellSize;
  const y = padding + move.row * cellSize;
  const radius = cellSize * 0.39;
  const gradient = context.createRadialGradient(
    x - radius * 0.35,
    y - radius * 0.4,
    2,
    x,
    y,
    radius,
  );

  if (move.player === 1) {
    gradient.addColorStop(0, "#625a55");
    gradient.addColorStop(0.52, "#302b28");
    gradient.addColorStop(1, "#171514");
  } else {
    gradient.addColorStop(0, "#ffffff");
    gradient.addColorStop(0.68, "#f5f0e8");
    gradient.addColorStop(1, "#d8cfc4");
  }

  context.save();
  context.shadowColor = "rgba(63, 43, 29, 0.28)";
  context.shadowBlur = 8;
  context.shadowOffsetY = 4;
  context.beginPath();
  context.arc(x, y, radius, 0, Math.PI * 2);
  context.fillStyle = gradient;
  context.fill();
  context.restore();

  context.beginPath();
  context.arc(x, y, radius, 0, Math.PI * 2);
  context.strokeStyle = move.player === 1 ? "#161412" : "#8f8479";
  context.lineWidth = 1.5;
  context.stroke();

  if (last) {
    context.beginPath();
    context.arc(x, y, 5, 0, Math.PI * 2);
    context.fillStyle = "#ff795f";
    context.fill();
  }
}

function handlePointer(event: PointerEvent) {
  if (props.disabled || !canvas.value) return;
  const bounds = canvas.value.getBoundingClientRect();
  const x = (event.clientX - bounds.left) * (internalSize / bounds.width);
  const y = (event.clientY - bounds.top) * (internalSize / bounds.height);
  const column = Math.round((x - padding) / cellSize);
  const row = Math.round((y - padding) / cellSize);
  if (row < 0 || row >= size || column < 0 || column >= size) return;
  if (props.board[row]?.[column]) {
    emit("occupied");
    return;
  }
  emit("move", row, column);
}

watch(
  () => [props.board, props.moves, props.winningLine],
  () => nextTick(draw),
  { deep: true },
);
onMounted(draw);
</script>

<template>
  <canvas
    ref="canvas"
    id="gomokuBoard"
    :width="internalSize"
    :height="internalSize"
    aria-label="十五乘十五五子棋棋盘"
    @pointerup="handlePointer"
  />
</template>
