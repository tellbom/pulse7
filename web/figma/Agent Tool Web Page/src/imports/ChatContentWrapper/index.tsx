import imgRectangle from "./23bd8a448f4f95d902efd08e55f2a9db274be45e.png";

export default function ChatContentWrapper() {
  return (
    <div className="bg-white content-stretch flex flex-col items-center overflow-clip p-[16px] relative rounded-[16px] size-full" data-name="Chat/Content Wrapper">
      <div className="h-[958px] relative shrink-0 w-[1191px]" data-name="Rectangle">
        <img alt="" className="absolute inset-0 max-w-none object-cover pointer-events-none size-full" src={imgRectangle} />
      </div>
    </div>
  );
}