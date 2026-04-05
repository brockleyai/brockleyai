import { useState } from "react";
import {
  BaseEdge,
  getSmoothStepPath,
  EdgeLabelRenderer,
  type EdgeProps,
} from "@xyflow/react";

export interface BrockleyEdgeData {
  backEdge?: boolean;
  isActive?: boolean;
  sourcePort?: string;
  targetPort?: string;
  onDelete?: () => void;
}

function BrockleyEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  markerEnd,
}: EdgeProps & { data?: BrockleyEdgeData }) {
  const [hovered, setHovered] = useState(false);

  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  let strokeColor = "rgba(255,255,255,0.15)";
  let strokeDasharray: string | undefined;
  let strokeOpacity = 1;
  const animatedClass = "";

  if (data?.backEdge) {
    strokeColor = "#818cf8";
    strokeDasharray = "6 4";
    strokeOpacity = 0.5;
  }

  if (data?.isActive) {
    strokeColor = "#818cf8";
    strokeDasharray = "6 4";
  }

  const portLabel =
    data?.sourcePort && data?.targetPort
      ? `${data.sourcePort} → ${data.targetPort}`
      : undefined;

  return (
    <>
      {/* Invisible wider path for hover detection */}
      <path
        d={edgePath}
        fill="none"
        stroke="transparent"
        strokeWidth={16}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
      />
      <BaseEdge
        id={id}
        path={edgePath}
        markerEnd={markerEnd}
        className={animatedClass}
        style={{
          stroke: strokeColor,
          strokeWidth: 2,
          strokeDasharray,
          opacity: strokeOpacity,
        }}
      />
      <EdgeLabelRenderer>
        {/* Port label */}
        {portLabel && (
          <div
            className="pointer-events-none absolute rounded bg-[#1a1d25]/90 px-1.5 py-0.5 text-[9px] text-gray-400"
            style={{
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
            }}
          >
            {portLabel}
          </div>
        )}

        {/* Delete button on hover */}
        {hovered && data?.onDelete && (
          <div
            className="absolute"
            style={{
              transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY - 16}px)`,
            }}
            onMouseEnter={() => setHovered(true)}
            onMouseLeave={() => setHovered(false)}
          >
            <button
              onClick={(e) => {
                e.stopPropagation();
                data.onDelete?.();
              }}
              className="flex h-5 w-5 items-center justify-center rounded-full border border-[rgba(255,255,255,0.08)] bg-[#1a1d25] text-[10px] text-gray-400 shadow-lg transition-colors hover:border-red-400 hover:text-red-400"
            >
              &times;
            </button>
          </div>
        )}
      </EdgeLabelRenderer>
    </>
  );
}

export default BrockleyEdge;
